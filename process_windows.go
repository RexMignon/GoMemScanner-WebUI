package main

import (
	"syscall"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	psapi    = syscall.NewLazyDLL("psapi.dll")

	enumProcesses      = psapi.NewProc("EnumProcesses")
	openProcess        = kernel32.NewProc("OpenProcess")
	getModuleBaseName  = psapi.NewProc("GetModuleBaseNameW")
	closeHandle        = kernel32.NewProc("CloseHandle")
	readProcessMemory  = kernel32.NewProc("ReadProcessMemory")
	writeProcessMemory = kernel32.NewProc("WriteProcessMemory")
	virtualQueryEx     = kernel32.NewProc("VirtualQueryEx")
)

const (
	PROCESS_ALL_ACCESS = 0x1F0FFF
	MAX_PATH           = 260
)

// GetProcessList 获取进程列表
func GetProcessList() ([]Process, error) {
	const maxProcesses = 1024
	processes := make([]uint32, maxProcesses)
	var needed uint32

	ret, _, err := enumProcesses.Call(
		uintptr(unsafe.Pointer(&processes[0])),
		uintptr(len(processes)*4),
		uintptr(unsafe.Pointer(&needed)),
	)

	if ret == 0 {
		return nil, err
	}

	numProcesses := needed / 4
	var result []Process

	for i := uint32(0); i < numProcesses; i++ {
		pid := processes[i]
		if pid == 0 {
			continue
		}

		handle, _, _ := openProcess.Call(
			PROCESS_ALL_ACCESS,
			0,
			uintptr(pid),
		)

		if handle == 0 {
			continue
		}

		name := make([]uint16, MAX_PATH)
		ret, _, _ = getModuleBaseName.Call(
			handle,
			0,
			uintptr(unsafe.Pointer(&name[0])),
			MAX_PATH,
		)

		closeHandle.Call(handle)

		if ret > 0 {
			result = append(result, Process{
				PID:  pid,
				Name: syscall.UTF16ToString(name[:ret]),
			})
		}
	}

	return result, nil
}

// OpenProcess 打开进程
func OpenProcess(pid uint32) (uintptr, error) {
	handle, _, err := openProcess.Call(
		PROCESS_ALL_ACCESS,
		0,
		uintptr(pid),
	)

	if handle == 0 {
		return 0, err
	}

	return handle, nil
}

// CloseProcess 关闭进程句柄
func CloseProcess(handle uintptr) {
	closeHandle.Call(handle)
}

// ReadMemory 读取内存
func ReadMemory(handle uintptr, address uintptr, size uint) ([]byte, error) {
	buffer := make([]byte, size)
	var read uintptr

	ret, _, err := readProcessMemory.Call(
		handle,
		address,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&read)),
	)

	if ret == 0 {
		return nil, err
	}

	return buffer, nil
}

// WriteMemory 写入内存
func WriteMemory(handle uintptr, address uintptr, data []byte) error {
	var written uintptr

	ret, _, err := writeProcessMemory.Call(
		handle,
		address,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		uintptr(unsafe.Pointer(&written)),
	)

	if ret == 0 {
		return err
	}

	return nil
}
