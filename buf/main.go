package main

import (
	"context"
	"sync"

	"github.com/avezila/buf/parser"
	"github.com/pkg/errors"
)

func main() {
	ms, err := MognoConnect()
	if err != nil {
		panic(errors.Wrap(err, "failed to connect mongo"))
	}
	parserContext := &parser.Context{
		Ctx: context.TODO(),
		MS:  ms,
	}
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := parser.JobFetchCompanys(parserContext)
		if err != nil {
			panic(errors.Wrap(err, "failed start JobFetchCompanys from main"))
		}
	}()
	// go func() {
	// 	defer wg.Done()
	// 	err := parser.StartServer(parserContext)
	// 	if err != nil {
	// 		panic(errors.Wrap(err, "failed start StartServer from main"))
	// 	}
	// }()
	wg.Wait()
}
