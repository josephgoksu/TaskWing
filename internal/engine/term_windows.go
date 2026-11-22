//go:build windows

package engine

import "os"

func termWidth(*os.File) int { return 0 }
