package main

import (
	"context"
	"fmt"
	"log"
	"os"

	gobunnings "github.com/MickMake/GoBunnings"
)

func main() {
	ctx := context.Background()

	ts, err := gobunnings.NewClientCredentialsTokenSource(
		gobunnings.EnvSandbox,
		os.Getenv("BUNNINGS_CLIENT_ID"),
		os.Getenv("BUNNINGS_CLIENT_SECRET"),
		[]string{"itm:details", "pri:pub", "inv:pub", "loc:pub", "ord:limited"},
	)
	if err != nil {
		log.Fatal(err)
	}

	client, err := gobunnings.New(gobunnings.EnvSandbox, ts)
	if err != nil {
		log.Fatal(err)
	}

	res, err := client.Item.Search(ctx, gobunnings.CountryAU, gobunnings.ItemSearchRequest{Query: "lawn"}, "")
	if err != nil {
		log.Fatal(err)
	}

	for _, item := range res.Results {
		fmt.Printf("%s\t%s\n", item.ItemNumber, item.Title)
	}
}
