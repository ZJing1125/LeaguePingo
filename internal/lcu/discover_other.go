//go:build !windows

package lcu

import "os"

func Discover() (*Client, error) {
	return DiscoverFromPaths([]string{os.Getenv("LEAGUEPINGO_LOCKFILE")})
}
