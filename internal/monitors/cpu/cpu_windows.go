// +build windows

package cpu

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/shirou/gopsutil/cpu"
	"golang.org/x/sys/windows"
)

// set gopsutil function to package variable for easier testing
var gopsutilTimes = cpu.Times

// getTimes function to a package variable to make it easier to test and
// override gopsutil on windows
var times = getTimes

var ntdll = windows.MustLoadDLL("Ntdll.dll") // NtQueryInformationProcess
var procNtQuerySystemInformation = ntdll.MustFindProc("NtQuerySystemInformation")

// SystemProcessorPerformanceInformation information class to query with NTQuerySystemInformation
const systemProcessorPerformanceInformation = 8

// SYSTEM_PROCESSOR_PERFORMANCE_INFORMATION
// defined in windows api doc with the following
// https://docs.microsoft.com/en-us/windows/desktop/api/winternl/nf-winternl-ntquerysysteminformation#system_processor_performance_information
// additional fields documented here
// https://www.geoffchappell.com/studies/windows/km/ntoskrnl/api/ex/sysinfo/processor_performance.htm
type SYSTEM_PROCESSOR_PERFORMANCE_INFORMATION struct {
	IdleTime       int64
	KernelTime     int64
	UserTime       int64
	DpcTime        int64
	InterruptTime  int64
	InterruptCount uint32
}

// converts the SYSTEM_PROCESSOR_PERFORMANCE_INFORMATION struct to a gopsutil cpu.TimesStat
func systemProcessorPerformanceInfoToCPUTimesStat(core int, s *SYSTEM_PROCESSOR_PERFORMANCE_INFORMATION) cpu.TimesStat {
	return cpu.TimesStat{
		CPU:    fmt.Sprintf("%d", core),
		Idle:   float64(s.IdleTime),
		System: float64(s.KernelTime - s.IdleTime),
		User:   float64(s.UserTime),
		Irq:    float64(s.InterruptTime),
	}
}

func getTimes(perCore bool) ([]cpu.TimesStat, error) {
	if !perCore {
		return gopsutilTimes(perCore)
	}
	// Underneathe hood gopsutil relies on a wmi query for per core cpu utilization information
	// this wmi query has proven to be problematic under unclear conditions.  It will hang
	// from time to time, and when executed frequently.  Many projects rely on gopsutil for this information.
	// Some have issues open complaining about hanging wmi calls, but none of a clear solution.
	// In general if you search for information about WMI it is the best of a bad situation.  It
	// is known to be buggy and slow.  A more performant solution is to
	// get the System Processor Performance Information from the ntQuerySystemInformationfunction.
	res, err := ntQuerySystemInformation()
	if err != nil {
		logger.Debugf("failed to execute ntQuerySystemInformation will try gopsutil")
		res, err = gopsutilTimes(perCore)
	}
	return res, err
}

// ntQuerySytemInformation gets percore cpu time information using the ntQuerySystemInformation windows api function.
// https://docs.microsoft.com/en-us/windows/desktop/api/winternl/nf-winternl-ntquerysysteminformation
// According to the windows documentation this is owned by the kernel and could go away in the future.
// However it has been around along time and the particular method we're using on the returned
// NtQuerySystemInformation has no recommended alternative yet.  If this ever breaks in future Windows
// versions, look at the help doc on ntquerysysteminformation and see if they've created an alternate
// api funciton to retrieve per core information.
func ntQuerySystemInformation() ([]cpu.TimesStat, error) {
	var coreInfo []cpu.TimesStat
	// Make maxResults large for safety.
	// We can't invoke the api call with a results array that's too small
	// If we have more than 2056 processors on a single host, then it's probably the future.
	maxResults := 2056
	pInfo := make([]SYSTEM_PROCESSOR_PERFORMANCE_INFORMATION, maxResults)
	size := uintptr(maxResults) * unsafe.Sizeof(pInfo[0])
	retCode, _, err := procNtQuerySystemInformation.Call(
		systemProcessorPerformanceInformation, // System Information Class => SystemProcessorPerformanceInformation
		uintptr(unsafe.Pointer(&pInfo[0])),    // System Information ARray for results
		size, // Length of the system information array
		uintptr(unsafe.Pointer(&size)), // Returned length of system information array
	)
	// err is not nil if the function succeeds so check the return code
	if retCode == 0 {
		err = nil
		// trim results to the known number of cores
		pInfo = pInfo[:runtime.NumCPU()]
		coreInfo = make([]cpu.TimesStat, 0, runtime.NumCPU())
		for core, info := range pInfo {
			coreInfo = append(coreInfo, systemProcessorPerformanceInfoToCPUTimesStat(core, &info))
		}
	}
	return coreInfo, err
}
