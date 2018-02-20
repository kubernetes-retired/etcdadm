package backupcontroller

import (
	"reflect"
	"testing"
	"time"
)

func TestParseBackup(t *testing.T) {
	grid := []struct {
		Input    string
		Expected *backupNameInfo
	}{
		{
			Input: "2018-02-20T04:47:55Z-000002",
			Expected: &backupNameInfo{
				Timestamp: time.Date(2018, 02, 20, 04, 47, 55, 0, time.UTC),
				Suffix:    "000002",
			},
		},
		{
			Input:    "",
			Expected: nil,
		},
		{
			Input:    "2018",
			Expected: nil,
		},
		{
			Input:    "Z-",
			Expected: nil,
		},
		{
			Input: "2018-02-20T04:47:55Z",
			Expected: &backupNameInfo{
				Timestamp: time.Date(2018, 02, 20, 04, 47, 55, 0, time.UTC),
				Suffix:    "",
			},
		},
		{
			Input: "2018-02-20T04:47:55Z-",
			Expected: &backupNameInfo{
				Timestamp: time.Date(2018, 02, 20, 04, 47, 55, 0, time.UTC),
				Suffix:    "",
			},
		},
	}

	for _, g := range grid {
		actual := parseBackupNameInfo(g.Input)

		if !reflect.DeepEqual(actual, g.Expected) {
			t.Errorf("unexpected parsed for %q: actual=%v expected=%v", g.Input, actual, g.Expected)
		}
	}

}
