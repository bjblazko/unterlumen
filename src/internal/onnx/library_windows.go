package onnx

import "syscall"

func openLibrary(path string) (uintptr, error) {
	h, err := syscall.LoadLibrary(path)
	return uintptr(h), err
}

func symbol(lib uintptr, name string) (uintptr, error) {
	return syscall.GetProcAddress(syscall.Handle(lib), name)
}
