//go:build !linux

package server

import (
	"context"
	"fmt"
	"palpanel/internal/docker"
)

func (m Manager) startLinux(context.Context, []string) error { return fmt.Errorf("linux_steamcmd runtime requires Linux host") }
func (m Manager) stopLinux(context.Context) error { return fmt.Errorf("linux_steamcmd runtime requires Linux host") }
func (m Manager) linuxStatus(context.Context) (docker.ContainerStatus, error) { return docker.ContainerStatus{Exists:false, Status:"missing"}, nil }
