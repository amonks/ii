package main

import (
	"fmt"
	"os"
	"sync"

	"monks.co/movietagger/db"
	"monks.co/movietagger/moviecopier"
	"monks.co/movietagger/movietagger"
	"monks.co/movietagger/system"
	"monks.co/movietagger/tmdb"
)

func main() {
	system := system.System{
		DB:   db.New("/mypool/tank/movies/.movies.db"),
		TMDB: tmdb.New("88f973483e2dc73cfb5053bc059ae33b"),
	}

	fmt.Printf("migrating...")
	if err := system.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf(" ok\n")

	mt := movietagger.New(system)
	mc := moviecopier.New(system)

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
}
