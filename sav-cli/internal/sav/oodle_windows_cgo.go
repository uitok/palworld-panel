//go:build windows && cgo

package sav

/*
#cgo LDFLAGS: -static -static-libgcc -static-libstdc++
*/
import "C"
