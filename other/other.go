package other

import (
	"fmt"
	"runtime"
)

func LogHeapData()  {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	fmt.Printf("\n\nHeap Alloc = %v MB\n", mem.HeapAlloc/1024/1024)
	fmt.Printf("Heap Sys = %v MB\n", mem.HeapSys/1024/1024)
	fmt.Printf("Heap Idel = %v MB\n", mem.HeapIdle/1024/1024)
	fmt.Printf("Heap Inuse = %v MB\n\n", mem.HeapInuse/1024/1024)
}