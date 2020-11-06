package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/blakesmith/ar"
	"github.com/ulikunitz/xz"
	"k8s.io/klog"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

type PackageExtractor struct {
	Out *tar.Writer

	Mirrors []string

	CacheDir string
}

func run(ctx context.Context) error {
	x := &PackageExtractor{}

	packageName := ""
	flag.StringVar(&packageName, "package", packageName, "name of package to extract")

	packagesPath := "Packages.gz"
	flag.StringVar(&packagesPath, "packages", packagesPath, "path to packages archive")

	packagesFormat := "gz"
	flag.StringVar(&packagesFormat, "packages-format", packagesFormat, "format of packages archive")

	outPath := ""
	flag.StringVar(&outPath, "out", outPath, "path to write extracted contents to")

	mirrors := ""
	flag.StringVar(&mirrors, "mirror", mirrors, "mirrors from which to download")

	flag.StringVar(&x.CacheDir, "cache-dir", x.CacheDir, "directory to use for caching artifacts")

	flag.Parse()

	if x.CacheDir == "" {
		// TODO: Clean up cache dir periodically to stop it growing indefinitely
		userCacheDir, err := os.UserCacheDir()
		if err != nil {
			klog.Warningf("unable to get cache dir; will use temp directory")
			tempDir := os.TempDir()
			userCacheDir = tempDir
		}
		x.CacheDir = userCacheDir
	}

	if packageName == "" {
		return fmt.Errorf("--package is required")
	}

	if outPath == "" {
		return fmt.Errorf("--out is required")
	}

	if mirrors == "" {
		return fmt.Errorf("--mirrors is required")
	}
	x.Mirrors = []string{mirrors}

	packages, err := parsePackages(packagesPath, packagesFormat)
	if err != nil {
		return err
	}

	packageInfo := packages[packageName]
	if packageInfo == nil {
		return fmt.Errorf("package %q not found", packageName)
	}

	//fileName := packageInfo.Filename
	//p := filepath.Base(fileName)

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("error creating output file %q: %w", outPath, err)
	}
	defer out.Close()

	tarWriter := tar.NewWriter(out)
	defer tarWriter.Close()

	x.Out = tarWriter

	return x.visitDeb(ctx, packageInfo)
}

func computeHashForFile(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	hash := hasher.Sum(nil)
	return hex.EncodeToString(hash), nil
}

func (x *PackageExtractor) downloadPackage(ctx context.Context, u string, expectedSHA256 string) (string, error) {
	cacheDir := filepath.Join(x.CacheDir, "deb-tools")
	if err := os.MkdirAll(cacheDir, os.FileMode(0755)); err != nil {
		return "", fmt.Errorf("error creating cache dir %q: %w", cacheDir, err)
	}

	cacheFile := filepath.Join(cacheDir, expectedSHA256)

	if hash, err := computeHashForFile(cacheFile); err == nil && hash == expectedSHA256 {
		return cacheFile, nil
	} else if err != nil && !os.IsNotExist(err) {
		klog.Warningf("failed to hash cache file %q: %w", cacheFile, err)
	} else if hash != expectedSHA256 && hash != "" {
		klog.Warningf("cache file %q did not have expected hash %q, was %q", cacheFile, expectedSHA256, hash)
	}

	f, err := ioutil.TempFile(cacheDir, "download")
	if err != nil {
		return "", fmt.Errorf("error creating cache file: %w", err)
	}
	defer f.Close()

	deleteTempFile := true
	defer func() {
		if deleteTempFile {
			if err := os.Remove(f.Name()); err != nil {
				klog.Warningf("failed to remove temp file %q: %w", f.Name(), err)
			}
		}
	}()

	response, err := http.Get(u)
	if err != nil {
		return "", fmt.Errorf("error fetching from %q: %w", u, err)
	}
	// f, err := os.Open(p)
	// if err != nil {
	// 	return fmt.Errorf("failed to open %q: %w", p, err)
	// }
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return "", fmt.Errorf("unexpected HTTP status fetching %q: %v", u, response.Status)
	}

	hasher := sha256.New()

	mw := io.MultiWriter(f, hasher)
	if _, err := io.Copy(mw, response.Body); err != nil {
		return "", fmt.Errorf("error downloading %q: %w", u, err)
	}

	hash := hasher.Sum(nil)
	actualSHA256 := hex.EncodeToString(hash)

	if actualSHA256 != expectedSHA256 {
		return "", fmt.Errorf("unexpected SHA256 for %q, got %q, expected %q", u, actualSHA256, expectedSHA256)
	}

	if err := os.Rename(f.Name(), cacheFile); err != nil {
		return "", fmt.Errorf("failed to rename temp file to cache file: %w", err)
	}

	// We've renamed it, deleting will fail
	deleteTempFile = false

	return cacheFile, nil
}

func (x *PackageExtractor) visitDeb(ctx context.Context, packageInfo *PackageInfo) error {
	u := x.Mirrors[0]
	if !strings.HasSuffix(u, "/") {
		u += "/"
	}
	u += strings.TrimPrefix(packageInfo.Filename, "/")

	p, err := x.downloadPackage(ctx, u, packageInfo.SHA256)
	if err != nil {
		return err
	}

	f, err := os.Open(p)
	if err != nil {
		return fmt.Errorf("error opening %q: %w", p, err)
	}
	defer f.Close()

	reader := ar.NewReader(f)
	for {
		header, err := reader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading from archive: %w", err)
		}
		if header.Name == "data.tar.xz" {
			xzReader, err := xz.NewReader(reader)
			if err != nil {
				return fmt.Errorf("error decompressing from xz: %w", err)
			}
			if err := x.visitDataTar(ctx, xzReader); err != nil {
				return fmt.Errorf("error reading %q: %w", header.Name, err)
			}
		}
		//klog.Infof("header: %+v", header)
	}
	return nil
}

func (x *PackageExtractor) visitDataTar(ctx context.Context, r io.Reader) error {
	tarReader := tar.NewReader(r)

	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading from tar: %w", err)
		}
		if err := x.visitPackageFile(ctx, header, tarReader); err != nil {
			return err
		}
	}
	return nil
}

func (x *PackageExtractor) visitPackageFile(ctx context.Context, header *tar.Header, r *tar.Reader) error {
	if err := x.Out.WriteHeader(header); err != nil {
		return fmt.Errorf("error writing header to output tar: %w", err)
	}

	if _, err := io.Copy(x.Out, r); err != nil {
		return fmt.Errorf("error copying file to output tar: %w", err)
	}

	return nil
}

// PackageInfo holds the per-package information parsed from a packages.gz file
type PackageInfo struct {
	Package  string
	Filename string
	SHA256   string
}

func parsePackages(p string, packagesFormat string) (map[string]*PackageInfo, error) {
	packages := make(map[string]*PackageInfo)

	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("error opening %q: %w", p, err)
	}
	defer f.Close()

	var r io.Reader
	switch packagesFormat {
	case "raw":
		r = f
	case "gz":
		gzipReader, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("error opening gzip stream for %q: %w", p, err)
		}
		defer gzipReader.Close()
		r = gzipReader
	default:
		return nil, fmt.Errorf("unknown packages format %q", packagesFormat)
	}

	scanner := bufio.NewScanner(r)
	currentPackage := &PackageInfo{}
	// We support multi-line keys
	key := ""

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, " ") {

		}

		if line == "" {
			if currentPackage.Package != "" {
				packages[currentPackage.Package] = currentPackage
			}
			currentPackage = &PackageInfo{}
			continue
		}

		var value string
		if strings.HasPrefix(line, " ") {
			value = line[1:]
		} else {
			tokens := strings.SplitN(line, ": ", 2)
			if len(tokens) != 2 {
				return nil, fmt.Errorf("cannot parse line %q", line)
			}
			key = tokens[0]
			value = tokens[1]
		}

		switch key {
		case "Package":
			currentPackage.Package += value
		case "Filename":
			currentPackage.Filename += value
		case "SHA256":
			currentPackage.SHA256 += value
		}
	}
	if currentPackage.Package != "" {
		packages[currentPackage.Package] = currentPackage
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading %q: %w", p, err)
	}

	return packages, nil
}
