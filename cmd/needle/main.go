package main

import (
	"flag"
	"fmt"
	"io/ioutil"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/stumble/needle/pkg/config"
	"github.com/stumble/needle/pkg/driver"
	"github.com/stumble/needle/pkg/passes"
	"github.com/stumble/needle/pkg/vcs"
)

func main() {
	genTemplate := flag.String("t", "", "generate a needle template")
	filePath := flag.String("f", "", "Input file path")
	outputPath := flag.String("o", "", "output file path")
	debug := flag.Bool("debug", false, "sets log level to debug")
	flag.Parse()

	log.Info().Msgf("needle version: %s", vcs.Commit)

	// Default level for this example is info, unless debug flag is present
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if *genTemplate != "" {
		if *outputPath == "" {
			panic("-o template filepath not provided ")
		}
		tmpl, err := config.GenTemplate(*genTemplate)
		if err != nil {
			panic(err)
		}
		err = ioutil.WriteFile(*outputPath, []byte(tmpl), 0600)
		if err != nil {
			panic(err)
		}
		return
	}

	if *filePath == "" {
		panic("filepath not provided")
	}

	config, err := config.ParseConfigFromFile(*filePath)
	if err != nil {
		panic(err)
	}

	repo, err := driver.NewRepoFromConfig(config)
	if err != nil {
		panic(err)
	}

	midend := &passes.NormalizePass{}
	err = midend.Run(repo)
	if err != nil {
		panic(err)
	}

	backend := &passes.CodegenPass{}
	err = backend.Run(repo)
	if err != nil {
		panic(err)
	}

	code := backend.Code
	if *outputPath == "" {
		fmt.Println(code)
	} else {
		err := ioutil.WriteFile(*outputPath, []byte(code), 0600)
		if err != nil {
			panic(err)
		}
	}
}
