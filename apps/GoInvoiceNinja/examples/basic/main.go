package main

import (
	"context"
	"fmt"
	"log"
	"os"

	ninja "github.com/MickMake/GoInvoiceNinja"
)

func main() {
	token := os.Getenv("INVOICE_NINJA_TOKEN")
	base := os.Getenv("INVOICE_NINJA_URL") // e.g. https://your-host.example.com or https://invoicing.co

	opts := []ninja.Option{}
	if base != "" {
		opts = append(opts, ninja.WithBaseURL(base))
	}

	c, err := ninja.New(token, opts...)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	clients, err := c.Clients.List(ctx, ninja.ClientQuery{
		ListOptions: ninja.ListOptions{PerPage: 10},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("clients: %d\n", len(clients.Data))

	invoices, err := c.Invoices.List(ctx, ninja.InvoiceQuery{
		ListOptions: ninja.ListOptions{PerPage: 10, Include: []string{"client"}},
		Payable:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, inv := range invoices.Data {
		fmt.Printf("%s balance %.2f client %s\n", inv.Number, inv.Balance, inv.ClientID)
	}
}
