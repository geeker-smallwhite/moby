package sysinfo // import "github.com/docker/docker/pkg/sysinfo"

import (
	"io/ioutil"
	"os"
	"path"
	"strings"

	cgroupsV2 "github.com/containerd/cgroups/v2"
	"github.com/containerd/containerd/pkg/userns"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/sirupsen/logrus"
)

func newV2(quiet bool, options ...Opt) *SysInfo {
	var warnings []string
	sysInfo := &SysInfo{
		CgroupUnified: true,
		cg2GroupPath:  "/",
	}
	for _, o := range options {
		o(sysInfo)
	}

	ops := []infoCollector{
		applyNetworkingInfo,
		applyAppArmorInfo,
		applySeccompInfo,
		applyCgroupNsInfo,
	}

	m, err := cgroupsV2.LoadManager("/sys/fs/cgroup", sysInfo.cg2GroupPath)
	if err != nil {
		logrus.Warn(err)
	} else {
		sysInfo.cg2Controllers = make(map[string]struct{})
		controllers, err := m.Controllers()
		if err != nil {
			logrus.Warn(err)
		}
		for _, c := range controllers {
			sysInfo.cg2Controllers[c] = struct{}{}
		}
		ops = append(ops,
			applyMemoryCgroupInfoV2,
			applyCPUCgroupInfoV2,
			applyIOCgroupInfoV2,
			applyCPUSetCgroupInfoV2,
			applyPIDSCgroupInfoV2,
			applyDevicesCgroupInfoV2,
		)
	}

	for _, o := range ops {
		w := o(sysInfo)
		warnings = append(warnings, w...)
	}
	if !quiet {
		for _, w := range warnings {
			logrus.Warn(w)
		}
	}
	return sysInfo
}

func getSwapLimitV2() bool {
	groups, err := cgroups.ParseCgroupFile("/proc/self/cgroup")
	if err != nil {
		return false
	}

	g := groups[""]
	if g == "" {
		return false
	}

	cGroupPath := path.Join("/sys/fs/cgroup", g, "memory.swap.max")
	if _, err = os.Stat(cGroupPath); os.IsNotExist(err) {
		return false
	}
	return true
}

func applyMemoryCgroupInfoV2(info *SysInfo) []string {
	var warnings []string
	if _, ok := info.cg2Controllers["memory"]; !ok {
		warnings = append(warnings, "Unable to find memory controller")
		return warnings
	}

	info.MemoryLimit = true
	info.SwapLimit = getSwapLimitV2()
	info.MemoryReservation = true
	info.OomKillDisable = false
	info.MemorySwappiness = false
	info.KernelMemory = false
	info.KernelMemoryTCP = false
	return warnings
}

func applyCPUCgroupInfoV2(info *SysInfo) []string {
	var warnings []string
	if _, ok := info.cg2Controllers["cpu"]; !ok {
		warnings = append(warnings, "Unable to find cpu controller")
		return warnings
	}
	info.CPUShares = true
	info.CPUCfs = true
	info.CPURealtime = false
	return warnings
}

func applyIOCgroupInfoV2(info *SysInfo) []string {
	var warnings []string
	if _, ok := info.cg2Controllers["io"]; !ok {
		warnings = append(warnings, "Unable to find io controller")
		return warnings
	}

	info.BlkioWeight = true
	info.BlkioWeightDevice = true
	info.BlkioReadBpsDevice = true
	info.BlkioWriteBpsDevice = true
	info.BlkioReadIOpsDevice = true
	info.BlkioWriteIOpsDevice = true
	return warnings
}

func applyCPUSetCgroupInfoV2(info *SysInfo) []string {
	var warnings []string
	if _, ok := info.cg2Controllers["cpuset"]; !ok {
		warnings = append(warnings, "Unable to find cpuset controller")
		return warnings
	}
	info.Cpuset = true

	cpus, err := ioutil.ReadFile(path.Join("/sys/fs/cgroup", info.cg2GroupPath, "cpuset.cpus.effective"))
	if err != nil {
		return warnings
	}
	info.Cpus = strings.TrimSpace(string(cpus))

	mems, err := ioutil.ReadFile(path.Join("/sys/fs/cgroup", info.cg2GroupPath, "cpuset.mems.effective"))
	if err != nil {
		return warnings
	}
	info.Mems = strings.TrimSpace(string(mems))
	return warnings
}

func applyPIDSCgroupInfoV2(info *SysInfo) []string {
	var warnings []string
	if _, ok := info.cg2Controllers["pids"]; !ok {
		warnings = append(warnings, "Unable to find pids controller")
		return warnings
	}
	info.PidsLimit = true
	return warnings
}

func applyDevicesCgroupInfoV2(info *SysInfo) []string {
	info.CgroupDevicesEnabled = !userns.RunningInUserNS()
	return nil
}
