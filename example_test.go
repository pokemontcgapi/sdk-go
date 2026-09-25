package pokemontcgapi_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/pokemontcgapi/sdk-go"
)

// ExampleClient needs PTCG_API_KEY in the environment; it is compiled, not run.
func ExampleClient() {
	ctx := context.Background()
	client := pokemontcgapi.New() // reads PTCG_API_KEY

	card, err := client.Cards.Get(ctx, "base1-4", &pokemontcgapi.CardGetParams{Include: []string{pokemontcgapi.IncludePrices}})
	if err != nil {
		var notFound *pokemontcgapi.NotFoundError
		if errors.As(err, &notFound) {
			log.Fatalf("no such card: %s", notFound.Message)
		}
		log.Fatal(err)
	}
	fmt.Println(card.ID, card.Name)

	page, err := client.Sets.List(ctx, &pokemontcgapi.SetListParams{Region: pokemontcgapi.RegionJP, Limit: 250})
	if err != nil {
		log.Fatal(err)
	}
	for set, err := range page.All(ctx) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(set.Code, set.Name)
	}
}
