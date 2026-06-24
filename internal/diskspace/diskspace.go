// Package diskspace provides free-space queries and a FATX on-disk footprint
// estimate, used by the LAN-saves download endpoint to refuse serving a save
// the target (Xbox HDD) or the server's staging area cannot hold.
//
// Two distinct checks live here because there are two "disks" in play:
//
//   - The Xbox FATX partition the nxdk client writes into. The client reports
//     its free bytes; FATXFootprint estimates how much the generated save set
//     will actually occupy once FATX rounds each file up to a cluster, and Fits
//     compares the two. This is the meaningful pre-flight: a half-written save
//     into a full FATX partition corrupts the directory.
//   - The server's own staging/output directory. Free(path) (statfs) guards
//     against generating into a server that is itself out of space.
package diskspace

import "syscall"

// DefaultFATXCluster is the cluster (allocation unit) size of a standard
// original-Xbox FATX data partition: 16 KiB. Every file occupies a whole number
// of clusters, so even a 26-byte SaveMeta.xbx consumes one full cluster.
const DefaultFATXCluster = 16 * 1024

// Free returns the number of bytes available to an unprivileged writer at path
// (the filesystem containing path). Uses the available-blocks count (Bavail),
// not the total free count, so reserved space is not counted as usable.
func Free(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return st.Bavail * uint64(st.Bsize), nil
}

// FATXFootprint estimates the on-disk bytes a set of files of the given sizes
// will occupy on a FATX partition with the given cluster size:
//
//   - each non-empty file rounds up to a whole number of clusters;
//   - one extra cluster is added for the save directory's own dirent entry/table.
//
// clusterSize <= 0 falls back to DefaultFATXCluster. The result is always a
// conservative over-estimate of the raw byte total, which is the safe side for
// a "will it fit?" guard.
func FATXFootprint(sizes []int, clusterSize int) uint64 {
	if clusterSize <= 0 {
		clusterSize = DefaultFATXCluster
	}
	cs := uint64(clusterSize)
	clusters := uint64(1) // the save directory itself
	for _, s := range sizes {
		if s <= 0 {
			continue
		}
		clusters += (uint64(s) + cs - 1) / cs
	}
	return clusters * cs
}

// Fits reports whether requiredBytes fit within availableBytes.
func Fits(requiredBytes, availableBytes uint64) bool {
	return requiredBytes <= availableBytes
}
