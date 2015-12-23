package main

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
)

func main() {
	dat, err := ioutil.ReadFile("/Users/pivotal/exp15.cleannums")
	if err != nil {
		panic(err)
	}
	dat_arr := make([]string, 100000)
	dat_arr = strings.Split(string(dat), "\n")

	var prev_val int64 = 0
	for i, val := range dat_arr {
		if len(val) == 0 {
			fmt.Println("\nDONE..")
			return
		}
		ival, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			fmt.Printf("\nIndex: %d; Value: %s", i, val)
			panic(err)
		}
		diff := ival - prev_val
		if diff != 1 {
			// do something
			fmt.Printf("\nGap Detected: Index: %d; Missing: %d; Value: %d", i, diff-1, ival)
		}
		prev_val = ival
	}

}
