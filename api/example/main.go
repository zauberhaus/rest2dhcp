package main

import (
	"context"
	"fmt"

	"github.com/zauberhaus/rest2dhcp/client"
)

func main() {
	cl := client.NewClient("http://localhost:8080")

	version, err := cl.Version(context.Background())
	if err != nil {
		panic(err)
	}

	fmt.Println(version)

	lease, err := cl.Lease(context.Background(), "test", nil)
	if err != nil {
		panic(err)
	}

	fmt.Println(lease)

	lease, err = cl.Renew(context.Background(), lease.Hostname, lease.Mac, lease.IP)
	if err != nil {
		panic(err)
	}

	fmt.Println(lease)

	err = cl.Release(context.Background(), lease.Hostname, lease.Mac, lease.IP)
	if err != nil {
		panic(err)
	}
}
