package main

import (
	"fmt"
	"os"

	"github.com/strongdm/kilroy/internal/server"
)

func attractorServe(args []string) {
	addr := ":8080"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--addr":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "--addr requires a value")
				os.Exit(1)
			}
			addr = args[i]
		default:
			fmt.Fprintf(os.Stderr, "unknown arg: %s\n", args[i])
			os.Exit(1)
		}
	}

	srv := server.New(server.Config{
		Addr: addr,
	})

	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
