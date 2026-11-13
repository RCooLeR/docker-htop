package sampler

import "iter"

// containerIDLen is the length of a full Docker container ID in hex characters.
const containerIDLen = 64

func isLowerHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f'
}

func isHex(c byte) bool {
	return isLowerHex(c) || c >= 'A' && c <= 'F'
}

// isContainerID reports whether s is a full container ID: 64 lowercase hex characters.
func isContainerID(s string) bool {
	if len(s) != containerIDLen {
		return false
	}
	for i := range len(s) {
		if !isLowerHex(s[i]) {
			return false
		}
	}
	return true
}

// containerIDs yields every run of exactly 64 lowercase hex characters in a cgroup file that is
// bounded by non-hex characters or the ends of the data. That covers /docker/<id>,
// /system.slice/docker-<id>.scope, cgroup v1 lines and /../<id>, the path a process shows when
// it is read from inside another cgroup namespace.
func containerIDs(data []byte) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		i := 0
		for i < len(data) {
			if !isHex(data[i]) {
				i++
				continue
			}
			j, lower := i, true
			for j < len(data) && isHex(data[j]) {
				lower = lower && isLowerHex(data[j])
				j++
			}
			if j-i == containerIDLen && lower && !yield(data[i:j]) {
				return
			}
			i = j
		}
	}
}

// firstContainerID returns the outermost container ID in a cgroup file, the one the Docker engine
// knows, or nil when the process is not in a container.
func firstContainerID(data []byte) []byte {
	for id := range containerIDs(data) {
		return id
	}
	return nil
}

// cgroupHasContainer reports whether containerID appears in a cgroup file.
func cgroupHasContainer(data []byte, containerID string) bool {
	for id := range containerIDs(data) {
		if string(id) == containerID {
			return true
		}
	}
	return false
}
