//go:build windows

package proxy

func killPIDs(pids []int) error {
	_ = pids
	return nil
}
