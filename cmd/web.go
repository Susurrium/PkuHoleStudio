package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var (
	webHost string
	webPort string
	webOpen bool
)

func newWebCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the local Web archive",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWeb(cmd.Context())
		},
	}
	cmd.Flags().StringVar(&webHost, "host", "127.0.0.1", "Web server host (loopback only)")
	cmd.Flags().StringVarP(&webPort, "port", "p", "8080", "Web server port")
	cmd.Flags().BoolVar(&webOpen, "open", true, "Open the Web client in the default browser")
	return cmd
}

func runWeb(ctx context.Context) error {
	listenHost, err := normalizeWebHost(webHost)
	if err != nil {
		return err
	}

	application, err := openApplication(ctx)
	if err != nil {
		return err
	}
	defer application.Close()
	router, err := newWebServerEngine(application)
	if err != nil {
		return fmt.Errorf("attach embedded Web client: %w", err)
	}
	address := net.JoinHostPort(listenHost, webPort)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}
	url := "http://" + net.JoinHostPort(listenHost, webPort)
	log.Printf("PkuHoleStudio Web is available at %s", url)
	if webOpen {
		if err := openBrowser(url); err != nil {
			log.Printf("Open the browser manually at %s (%v)", url, err)
		}
	}
	if err := router.RunListener(listener); err != nil {
		return fmt.Errorf("serve Web client: %w", err)
	}
	return nil
}

func normalizeWebHost(host string) (string, error) {
	normalized := strings.TrimSpace(host)
	if strings.HasPrefix(normalized, "[") || strings.HasSuffix(normalized, "]") {
		if len(normalized) < 2 || !strings.HasPrefix(normalized, "[") || !strings.HasSuffix(normalized, "]") {
			return "", fmt.Errorf("refusing Web host %q: Web is local-only; use localhost or a loopback IP address", host)
		}
		normalized = strings.TrimSpace(normalized[1 : len(normalized)-1])
	}
	if normalized == "" || strings.ContainsAny(normalized, "[]") || !isLoopbackServerHost(normalized) {
		return "", fmt.Errorf("refusing Web host %q: Web is local-only; use localhost or a loopback IP address", host)
	}
	if strings.EqualFold(normalized, "localhost") {
		return "127.0.0.1", nil
	}
	return normalized, nil
}

func openBrowser(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		command = exec.Command("open", url)
	default:
		command = exec.Command("xdg-open", url)
	}
	return command.Start()
}
