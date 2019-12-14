/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package volumes

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	utilexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"
	"k8s.io/utils/nsenter"
	"kope.io/etcd-manager/pkg/hostmount"
)

type VolumeMountController struct {
	mounted map[string]*Volume

	provider Volumes
}

func newVolumeMountController(provider Volumes) *VolumeMountController {
	c := &VolumeMountController{}
	c.mounted = make(map[string]*Volume)
	c.provider = provider
	return c
}

func (k *VolumeMountController) mountMasterVolumes() ([]*Volume, error) {
	// TODO: mount ephemeral volumes (particular on AWS)?

	// Mount master volumes
	attached, err := k.attachMasterVolumes()
	if err != nil {
		return nil, fmt.Errorf("unable to attach master volumes: %v", err)
	}

	for _, v := range attached {
		if len(k.mounted) > 0 {
			// We only attempt to mount a single volume
			break
		}

		existing := k.mounted[v.ProviderID]
		if existing != nil {
			continue
		}

		glog.V(2).Infof("Master volume %q is attached at %q", v.ProviderID, v.LocalDevice)

		mountpoint := "/mnt/" + v.MountName

		// On ContainerOS, we mount to /mnt/disks instead (/mnt is readonly)
		_, err := os.Stat(PathFor("/mnt/disks"))
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("error checking for /mnt/disks: %v", err)
			}
		} else {
			mountpoint = "/mnt/disks/" + v.MountName
		}

		glog.Infof("Doing safe-format-and-mount of %s to %s", v.LocalDevice, mountpoint)
		fstype := ""
		err = k.safeFormatAndMount(v, mountpoint, fstype)
		if err != nil {
			glog.Warningf("unable to mount master volume: %q", err)
			continue
		}

		glog.Infof("mounted master volume %q on %s", v.ProviderID, mountpoint)

		v.Mountpoint = PathFor(mountpoint)
		k.mounted[v.ProviderID] = v
	}

	var volumes []*Volume
	for _, v := range k.mounted {
		volumes = append(volumes, v)
	}
	return volumes, nil
}

func (k *VolumeMountController) safeFormatAndMount(volume *Volume, mountpoint string, fstype string) error {
	// Wait for the device to show up; we currently wait forever with naive retry logic
	device := ""
	for {
		found, err := k.provider.FindMountedVolume(volume)
		if err != nil {
			return err
		}

		if found != "" {
			device = found
			break
		}

		glog.Infof("Waiting for volume %q to be mounted", volume.ProviderID)
		// TODO: Some form of backoff?
		time.Sleep(1 * time.Second)
	}
	glog.Infof("Found volume %q mounted at device %q", volume.ProviderID, device)

	safeFormatAndMount := &mount.SafeFormatAndMount{}

	if Containerized {
		ne, err := nsenter.NewNsenter(PathFor("/"), utilexec.New())
		if err != nil {
			return fmt.Errorf("error building ns-enter helper: %v", err)
		}

		// Build mount & exec implementations that execute in the host namespaces
		safeFormatAndMount.Interface = hostmount.New(ne)
		safeFormatAndMount.Exec = ne

		// Note that we don't use PathFor for operations going through safeFormatAndMount,
		// because NewNsenterMounter and NewNsEnterExec will operate in the host
	} else {
		safeFormatAndMount.Interface = mount.New("")
		safeFormatAndMount.Exec = utilexec.New()
	}

	// Check if it is already mounted
	// TODO: can we now use IsLikelyNotMountPoint or IsMountPointMatch instead here
	mounts, err := safeFormatAndMount.List()
	if err != nil {
		return fmt.Errorf("error listing existing mounts: %v", err)
	}

	var existing []*mount.MountPoint
	for i := range mounts {
		m := &mounts[i]
		glog.V(8).Infof("found existing mount: %v", m)
		// Note: when containerized, we still list mounts in the host, so we don't need to call PathFor(mountpoint)
		if m.Path == mountpoint {
			existing = append(existing, m)
		}
	}

	// Mount only if isn't mounted already
	if len(existing) == 0 {
		options := []string{}

		glog.Infof("Creating mount directory %q", PathFor(mountpoint))
		if err := os.MkdirAll(PathFor(mountpoint), 0750); err != nil {
			return err
		}

		glog.Infof("Mounting device %q on %q", device, mountpoint)

		err = safeFormatAndMount.FormatAndMount(device, mountpoint, fstype, options)
		if err != nil {
			return fmt.Errorf("error formatting and mounting disk %q on %q: %v", device, mountpoint, err)
		}
	} else {
		glog.Infof("Device already mounted on %q, verifying it is our device", mountpoint)

		if len(existing) != 1 {
			glog.Infof("Existing mounts unexpected")

			for i := range mounts {
				m := &mounts[i]
				glog.Infof("%s\t%s", m.Device, m.Path)
			}

			return fmt.Errorf("found multiple existing mounts of %q at %q", device, mountpoint)
		} else {
			glog.Infof("Found existing mount of %q at %q", device, mountpoint)
		}
	}

	// If we're containerized we also want to mount the device (again) into our container
	// We could also do this with mount propagation, but this is simple
	if Containerized {
		source := PathFor(device)
		target := PathFor(mountpoint)
		options := []string{}

		mounter := mount.New("")

		mountedDevice, _, err := mount.GetDeviceNameFromMount(mounter, target)
		if err != nil {
			return fmt.Errorf("error checking for mounts of %s inside container: %v", target, err)
		}

		if mountedDevice != "" {
			if !isSameDevice(device, mountedDevice) {
				return fmt.Errorf("device already mounted at %s, but is %s and we want %s or %s", target, mountedDevice, source, device)
			}
		} else {
			glog.Infof("mounting inside container: %s -> %s", source, target)
			if err := mounter.Mount(source, target, fstype, options); err != nil {
				return fmt.Errorf("error mounting %s inside container at %s: %v", source, target, err)
			}
		}
	}

	return nil
}

// Checks if l and r are the same device.  We tolerate /dev/X vs /rootfs/dev/X, and we also dereference symlinks
func isSameDevice(dev1, dev2 string) bool {
	aliases1 := namesFor(dev1)
	aliases2 := namesFor(dev2)

	for k1 := range aliases1 {
		for k2 := range aliases2 {
			if k1 == k2 {
				glog.V(2).Infof("matched device %q and %q via %q", dev1, dev2, k1)
				return true
			}
		}
	}

	glog.Warningf("device %q and %q were not the same; expansions %v and %v", dev1, dev2, aliases1, aliases2)

	return false
}

// namesFor tries to find all the names for the device, including dereferencing symlinks and looking for /rootfs alias
func namesFor(dev string) []string {
	var names []string
	names = append(names, dev)
	names = append(names, PathFor(dev))

	// Check for a symlink
	s, err := filepath.EvalSymlinks(dev)
	if err != nil {
		glog.Infof("device %q did not evaluate as a symlink: %v", dev, err)
	} else {
		a, err := filepath.Abs(s)
		if err != nil {
			glog.Warningf("unable to make filepath %q absolute: %v", s, err)
		} else {
			s = a
		}
		names = append(names, s)
	}

	return names
}

func (k *VolumeMountController) attachMasterVolumes() ([]*Volume, error) {
	volumes, err := k.provider.FindVolumes()
	if err != nil {
		return nil, err
	}

	var tryAttach []*Volume
	var attached []*Volume
	for _, v := range volumes {
		if v.AttachedTo == "" {
			tryAttach = append(tryAttach, v)
		}
		if v.LocalDevice != "" {
			attached = append(attached, v)
		}
	}

	if len(tryAttach) == 0 {
		return attached, nil
	}

	// Actually attempt the mounting
	for _, v := range tryAttach {
		if len(attached) > 0 {
			// We only attempt to mount a single volume
			break
		}

		glog.V(2).Infof("Trying to mount master volume: %q", v.ProviderID)

		err := k.provider.AttachVolume(v)
		if err != nil {
			// We are racing with other instances here; this can happen
			glog.Warningf("Error attaching volume %q: %v", v.ProviderID, err)
		} else {
			if v.LocalDevice == "" {
				glog.Fatalf("AttachVolume did not set LocalDevice")
			}
			attached = append(attached, v)
		}
	}

	glog.V(2).Infof("Currently attached volumes: %v", attached)
	return attached, nil
}
