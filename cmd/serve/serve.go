package serve

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/Owloops/tfjournal/server"
	"github.com/Owloops/tfjournal/storage"
)

const (
	_defaultPort = 8080
	_defaultBind = "127.0.0.1"
)

var (
	_port     int
	_bindAddr string
)

var Cmd = &cobra.Command{
	Use:   "serve",
	Short: "Start web UI server",
	Long:  "Launch HTTP server to browse terraform runs in a web interface",
	RunE:  runServe,
}

func init() {
	Cmd.Flags().IntVarP(&_port, "port", "p", _defaultPort, "Port to listen on")
	Cmd.Flags().StringVarP(&_bindAddr, "bind", "b", _defaultBind, "Address to bind to")
}

func SetVersion(v string) {
	server.Version = v
}

func runServe(cmd *cobra.Command, args []string) error {
	store, err := storage.NewFromEnv()
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer func() { _ = store.Close() }()

	port := _port
	bind := _bindAddr

	if !cmd.Flags().Changed("port") {
		if envPort := os.Getenv("TFJOURNAL_PORT"); envPort != "" {
			if p, err := strconv.Atoi(envPort); err == nil {
				port = p
			}
		}
	}

	if !cmd.Flags().Changed("bind") {
		if envBind := os.Getenv("TFJOURNAL_BIND"); envBind != "" {
			bind = envBind
		}
	}

	srv := server.New(store)
	addr := fmt.Sprintf("%s:%d", bind, port)

	username := os.Getenv("TFJOURNAL_USERNAME")
	password := os.Getenv("TFJOURNAL_PASSWORD")

	var handler http.Handler = srv
	if username != "" && password != "" {
		handler = srv.WithBasicAuth(username, password)
		fmt.Fprintf(os.Stderr, "tfjournal web ui: http://%s (basic auth enabled)\n", addr)
	} else {
		fmt.Fprintf(os.Stderr, "tfjournal web ui: http://%s\n", addr)
	}

	return http.ListenAndServe(addr, handler)
}
