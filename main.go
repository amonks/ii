package main

import (
	"fmt"
	"os"
	"sync"

	"monks.co/movietagger/db"
	"monks.co/movietagger/moviecopier"
	"monks.co/movietagger/movietagger"
	"monks.co/movietagger/tmdb"
)

func main() {
	tmdb := tmdb.New("88f973483e2dc73cfb5053bc059ae33b")
	db := db.New(".movies.db")

	fmt.Printf("migrating...")
	if err := db.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf(" ok\n")

	mt := movietagger.New(tmdb, db)
	mc := moviecopier.New(db)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		fmt.Println("moviecopier: start")
		if err := mc.Run(); err != nil {
			fmt.Println(err)
		}
		fmt.Println("moviecopier: done")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		fmt.Println("movietagger: start")
		if err := mt.Run(); err != nil {
			fmt.Println(err)
		}
		fmt.Println("movietagger: done")
	}()

	wg.Wait()

	fmt.Println("moviecopier: second pass")
	if err := mc.Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("all done.")
}
