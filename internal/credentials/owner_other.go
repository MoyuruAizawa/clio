//go:build !darwin && !linux

package credentials

import "os"

func owned(s os.FileInfo) bool { return false }
