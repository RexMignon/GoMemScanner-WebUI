package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"unsafe"
)

// MemoryRegion 内存区域
type MemoryRegion struct {
	BaseAddress uintptr
	Size        uint
}

// MemoryResult 搜索结果
type MemoryResult struct {
	Address string `json:"address"`
	Value   string `json:"value"`
}

// MEMORY_BASIC_INFORMATION Windows内存信息结构
type MEMORY_BASIC_INFORMATION struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
}

const (
	MEM_COMMIT     = 0x1000
	PAGE_READWRITE = 0x04
)

// ScanMemory 扫描内存
func ScanMemory(pid uint32, value interface{}, dataType string, operation string, previousResults []MemoryResult) ([]MemoryResult, error) {
	handle, err := OpenProcess(pid)
	if err != nil {
		return nil, err
	}
	defer CloseProcess(handle)

	var results []MemoryResult
	searchData, err := convertValueToBytes(value, dataType)
	if err != nil {
		return nil, err
	}

	// 如果有之前的结果，只扫描这些地址
	if len(previousResults) > 0 {
		for _, prev := range previousResults {
			var addr uint64
			fmt.Sscanf(prev.Address, "0x%X", &addr)

			memory, err := ReadMemory(handle, uintptr(addr), uint(len(searchData)))
			if err != nil {
				continue
			}

			if operation == "increased" {
				prevValue := getValue(memory, dataType)
				if prevValue > getValue(searchData, dataType) {
					results = append(results, MemoryResult{
						Address: prev.Address,
						Value:   formatValue(memory, dataType),
					})
				}
			} else if operation == "decreased" {
				prevValue := getValue(memory, dataType)
				if prevValue < getValue(searchData, dataType) {
					results = append(results, MemoryResult{
						Address: prev.Address,
						Value:   formatValue(memory, dataType),
					})
				}
			} else {
				matches := findMatches(memory, searchData, dataType, operation)
				if len(matches) > 0 {
					results = append(results, MemoryResult{
						Address: prev.Address,
						Value:   formatValue(memory, dataType),
					})
				}
			}
		}
		return results, nil
	}

	// 首次搜索，扫描所有内存区域
	regions, err := getMemoryRegions(handle)
	if err != nil {
		return nil, err
	}

	for _, region := range regions {
		memory, err := ReadMemory(handle, region.BaseAddress, region.Size)
		if err != nil {
			continue
		}

		matches := findMatches(memory, searchData, dataType, operation)
		for _, offset := range matches {
			addr := region.BaseAddress + uintptr(offset)
			val := formatValue(memory[offset:], dataType)
			results = append(results, MemoryResult{
				Address: fmt.Sprintf("0x%X", addr),
				Value:   val,
			})
		}
	}

	return results, nil
}

// getMemoryRegions 获取可读写的内存区域
func getMemoryRegions(handle uintptr) ([]MemoryRegion, error) {
	var regions []MemoryRegion
	var address uintptr = 0

	for {
		var mbi MEMORY_BASIC_INFORMATION
		ret, _, _ := virtualQueryEx.Call(
			handle,
			address,
			uintptr(unsafe.Pointer(&mbi)),
			unsafe.Sizeof(mbi),
		)

		if ret == 0 {
			break
		}

		if mbi.State&MEM_COMMIT != 0 && mbi.Protect&PAGE_READWRITE != 0 {
			regions = append(regions, MemoryRegion{
				BaseAddress: mbi.BaseAddress,
				Size:        uint(mbi.RegionSize),
			})
		}

		address = mbi.BaseAddress + uintptr(mbi.RegionSize)
	}

	return regions, nil
}

// convertValueToBytes 将值转换为字节数组
func convertValueToBytes(value interface{}, dataType string) ([]byte, error) {
	buf := new(bytes.Buffer)
	var err error

	switch dataType {
	case "int32":
		v, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("invalid value type for int32")
		}
		err = binary.Write(buf, binary.LittleEndian, int32(v))
	case "int64":
		v, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("invalid value type for int64")
		}
		err = binary.Write(buf, binary.LittleEndian, int64(v))
	case "float32":
		v, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("invalid value type for float32")
		}
		err = binary.Write(buf, binary.LittleEndian, float32(v))
	case "float64":
		v, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("invalid value type for float64")
		}
		err = binary.Write(buf, binary.LittleEndian, v)
	default:
		return nil, fmt.Errorf("unsupported data type")
	}

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// findMatches 在内存中查找匹配
func findMatches(memory []byte, searchData []byte, dataType string, operation string) []int {
	var matches []int
	searchLen := len(searchData)

	for i := 0; i <= len(memory)-searchLen; i++ {
		if operation == "equal" {
			if bytes.Equal(memory[i:i+searchLen], searchData) {
				matches = append(matches, i)
			}
		} else {
			currentValue := getValue(memory[i:], dataType)
			searchValue := getValue(searchData, dataType)

			switch operation {
			case "greater":
				if currentValue > searchValue {
					matches = append(matches, i)
				}
			case "less":
				if currentValue < searchValue {
					matches = append(matches, i)
				}
			}
		}
	}

	return matches
}

// getValue 从字节数组中获取值
func getValue(data []byte, dataType string) float64 {
	switch dataType {
	case "int32":
		return float64(int32(binary.LittleEndian.Uint32(data)))
	case "int64":
		return float64(int64(binary.LittleEndian.Uint64(data)))
	case "float32":
		bits := binary.LittleEndian.Uint32(data)
		return float64(math.Float32frombits(bits))
	case "float64":
		bits := binary.LittleEndian.Uint64(data)
		return math.Float64frombits(bits)
	default:
		return 0
	}
}

// formatValue 格式化值为字符串
func formatValue(data []byte, dataType string) string {
	value := getValue(data, dataType)
	switch dataType {
	case "int32", "int64":
		return fmt.Sprintf("%d", int64(value))
	case "float32", "float64":
		return fmt.Sprintf("%f", value)
	default:
		return fmt.Sprintf("%v", value)
	}
}

// ModifyMemory 修改内存值
func ModifyMemory(pid uint32, address string, value interface{}, dataType string) error {
	handle, err := OpenProcess(pid)
	if err != nil {
		return err
	}
	defer CloseProcess(handle)

	// 将地址字符串转换为uintptr
	var addr uint64
	fmt.Sscanf(address, "0x%X", &addr)

	data, err := convertValueToBytes(value, dataType)
	if err != nil {
		return err
	}

	return WriteMemory(handle, uintptr(addr), data)
}
