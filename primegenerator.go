// Copyright (C) 2017 by Max Ungless
// Main package for primegenerator. Generates primes.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"sort"
	"sync/atomic"
	"time"

	"github.com/MaxTheMonster/PrimeNumberGenerator/client"
	"github.com/MaxTheMonster/PrimeNumberGenerator/primes"
	"github.com/MaxTheMonster/PrimeNumberGenerator/server"
	"github.com/urfave/cli"
)

var lastPrimeGenerated *big.Int

type bigIntSlice []*big.Int

func (s bigIntSlice) Len() int           { return len(s) }
func (s bigIntSlice) Less(i, j int) bool { return s[i].Cmp(s[j]) < 0 }
func (s bigIntSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// formatFilePath formats inputted filename to create a proper file path.
func formatFilePath(filename string) string {
	return base + filename + ".txt"
}

// showHelp shows help to the user.
func showHelp() {
	fmt.Println("COMMANDS")
	fmt.Println("count \t Displays the total number of generated primes.")
	fmt.Println("configure \t Generates a configuration for the program.")
	fmt.Println("run \t Runs the program indefinitely.")
	fmt.Println("version \t Displays the version of the program.")
	fmt.Println("help \t Displays this screen. Gives help.")

	os.Exit(1)
}

// showProgramDetails prints details about the program to STDOUT
func showProgramDetails() {
	fmt.Printf("PrimeNumberGenerator %s", version)
	fmt.Println("\nCopyright (C) 2017 by Max Ungless")
	fmt.Println("This program comes with ABSOLUTELY NO WARRANTY.\nThis is free software, and you are welcome to redistribute it\nunder the condiditions set in the GNU General Public License version 3.\nSee the file named LICENSE for details.")
	fmt.Println("\nFor bugs, send mail to max@maxungless.com")
	fmt.Println()
}

// getLastPrime() searches for last generated prime
// in all prime storage files.
func getLastPrime() *big.Int {
	latestFile := OpenLatestFile(os.O_RDONLY, 0666)
	defer latestFile.Close()

	var lastPrimeGenerated string
	scanner := bufio.NewScanner(latestFile)
	for scanner.Scan() {
		lastPrimeGenerated = scanner.Text()
	}
	if lastPrimeGenerated == "0" || lastPrimeGenerated == "" {
		lastPrimeGenerated = startingPrime
	}
	foundPrime := new(big.Int)
	foundPrime.SetString(lastPrimeGenerated, 10)
	return foundPrime
}

// Round() is used to round numbers to the nearest x
func Round(x, unit float64) float64 {
	return float64(int64(x/unit+0.5)) * unit
}

// convertPrimesToWritableFormat() takes a buffer of primes and converts them to a string
// with each prime separated by a newline
func convertPrimesToWritableFormat(buffer []*big.Int) string {
	var formattedBuffer bytes.Buffer
	for _, prime := range buffer {
		formattedBuffer.WriteString(prime.String() + "\n")
	}
	return formattedBuffer.String()
}

// FlushBufferToFile() takes a buffer of primes and flushes them to the latest file
func FlushBufferToFile(buffer bigIntSlice) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Println("Writing buffer....")
	sort.Sort(buffer)
	atomic.AddUint64(&id, uint64(maxBufferSize))

	file := OpenLatestFile(os.O_APPEND|os.O_WRONLY, 0600)
	defer file.Close()
	readableBuffer := convertPrimesToWritableFormat(buffer)

	file.WriteString(readableBuffer)
	fmt.Println("Finished writing buffer.")
}

// ComputePrimes computes primes concurrently until KeyboardInterrupt
func ComputePrimes(lastPrime *big.Int, writeToFile bool, toInfinity bool, maxNumber *big.Int) {
	numbersToCheck := make(chan *big.Int, 100)
	validPrimes := make(chan primes.Prime, 100)
	invalidPrimes := make(chan primes.Prime, 100)
	var primeBuffer bigIntSlice

	go func() {
		if toInfinity {
			for i := lastPrime; true; i.Add(i, big.NewInt(2)) {
				numberToTest := big.NewInt(0).Set(i)
				numbersToCheck <- numberToTest
			}
		} else {
			for i := lastPrime; i.Cmp(maxNumber) == -1; i.Add(i, big.NewInt(2)) {
				numberToTest := big.NewInt(0).Set(i)
				numbersToCheck <- numberToTest
			}
		}
	}()

	go func() {
		for elem := range validPrimes {
			primeBuffer = append(primeBuffer, elem.Value)
			if len(primeBuffer) == maxBufferSize {
				if writeToFile {
					FlushBufferToFile(primeBuffer)
				}
				primeBuffer = nil
			}
			primes.DisplayPrimePretty(elem.Value, elem.TimeTaken)
		}
	}()

	go func() {
		for elem := range invalidPrimes {
			if showFails == true {
				primes.DisplayFailPretty(elem.Value, elem.TimeTaken)
			}
		}
	}()

	for i := range numbersToCheck {
		go func(i *big.Int) {
			start := time.Now()
			isPrime := primes.CheckPrimality(i)
			if isPrime == true {
				validPrimes <- primes.Prime{
					TimeTaken: time.Now().Sub(start),
					Value:     i,
					Id:        id,
				}
			} else {
				invalidPrimes <- primes.Prime{
					TimeTaken: time.Now().Sub(start),
					Value:     i,
				}
			}
		}(i)
	}
}

func init() {
	logger.SetOutput(ioutil.Discard)
	showProgramDetails()
	SetConfiguration()
}

func main() {
	app := cli.NewApp()
	app.Name = appName
	app.Usage = appUsage
	app.Version = version

	app.Commands = []cli.Command{
		{
			Name:    "configure",
			Aliases: []string{"cn"},
			Usage:   descConfigure,
			Action: func(c *cli.Context) error {
				RunConfigurator()
				return nil
			},
		},
		{
			Name:    "count",
			Aliases: []string{"ct"},
			Usage:   descCount,
			Before: func(c *cli.Context) error {
				SetId()
				return nil
			},
			Action: func(c *cli.Context) error {
				ShowCurrentCount()
				return nil
			},
		},
		{
			Name:    "run",
			Aliases: []string{"r"},
			Usage:   descRun,
			Before: func(c *cli.Context) error {
				SetId()
				SetLastPrimeGenerated()
				return nil
			},
			Action: func(c *cli.Context) error {
				ComputePrimes(lastPrimeGenerated, true, true, big.NewInt(0))
				return nil
			},
		},
		{
			Name:    "client",
			Aliases: []string{"cl"},
			Usage:   descClient,
			Action:  client.LaunchClient,
		},
		{
			Name:    "server",
			Aliases: []string{"s"},
			Usage:   descServer,
			Before: func(c *cli.Context) error {
				SetId()
				return nil
			},
			Action: server.LaunchServer,
		},
	}
	app.Run(os.Args)
}
