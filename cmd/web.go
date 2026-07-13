package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"runtime"

	"github.com/Susurrium/PkuHoleStudio/internal/app"

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
	cmd.Flags().StringVar(&webHost, "host", "127.0.0.1", "Web server host")
	cmd.Flags().StringVarP(&webPort, "port", "p", "8080", "Web server port")
	cmd.Flags().BoolVar(&webOpen, "open", true, "Open the Web client in the default browser")
	return cmd
}

func runWeb(ctx context.Context) error {
	application, err := app.Open(ctx, app.Options{})
	if err != nil {
		return err
	}
	defer application.Close()
	router, err := newWebServerEngine(application)
	if err != nil {
		return fmt.Errorf("attach embedded Web client: %w", err)
	}
	address := net.JoinHostPort(webHost, webPort)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}
	urlHost := webHost
	if urlHost == "0.0.0.0" || urlHost == "::" {
		urlHost = "127.0.0.1"
	}
	url := "http://" + net.JoinHostPort(urlHost, webPort)
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
