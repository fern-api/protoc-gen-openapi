// Copyright 2020 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fern-api/protoc-gen-openapi/cmd/protoc-gen-openapi/generator"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

var flags flag.FlagSet

func main() {
	conf := generator.Configuration{
		Version:         flags.String("version", "0.0.1", "version number text, e.g. 1.2.3"),
		Title:           flags.String("title", "", "name of the API"),
		Description:     flags.String("description", "", "description of the API"),
		Naming:          flags.String("naming", "proto", `naming convention. Use "proto" for passing names directly from the proto files`),
		FQSchemaNaming:  flags.Bool("fq_schema_naming", false, `schema naming convention. If "true", generates fully-qualified schema names by prefixing them with the proto message package name`),
		EnumType:        flags.String("enum_type", "integer", `type for enum serialization. Use "string" for string-based serialization`),
		CircularDepth:   flags.Int("depth", 2, "depth of recursion for circular messages"),
		DefaultResponse: flags.Bool("default_response", true, `add default response. If "true", automatically adds a default response to operations which use the google.rpc.Status message. Useful if you use envoy or grpc-gateway to transcode as they use this type for their default error responses.`),
		OutputMode:      flags.String("output_mode", "merged", `output generation mode. By default, a single openapi.yaml is generated at the out folder. Use "source_relative' to generate a separate '[inputfile].openapi.yaml' next to each '[inputfile].proto'.`),
		SourceRoot:      flags.String("source_root", "", `root directory of the source Protobuf files. This is used to add source information to the openapi.yaml file.`),
		FlattenOneofs:   flags.Bool("flatten_oneofs", false, `flatten oneof fields as regular properties on the parent message instead of generating a oneOf union`),
	}

	opts := protogen.Options{
		ParamFunc: flags.Set,
	}

	if err := run(opts, conf); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", filepath.Base(os.Args[0]), err)
		os.Exit(1)
	}
}

func run(opts protogen.Options, conf generator.Configuration) error {
	if len(os.Args) > 1 {
		return fmt.Errorf("unknown argument %q (this program should be run by protoc, not directly)", os.Args[1])
	}
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(in, req); err != nil {
		return err
	}
	// The protogen library requires every .proto file to have a go_package option,
	// but this plugin generates OpenAPI specs — not Go code. Inject a synthetic
	// go_package for any file that doesn't already have one so users aren't forced
	// to add a Go-specific option to their proto files.
	for _, fdesc := range req.ProtoFile {
		if fdesc.GetOptions().GetGoPackage() == "" {
			if fdesc.Options == nil {
				fdesc.Options = &descriptorpb.FileOptions{}
			}
			pkg := fdesc.GetPackage()
			if pkg == "" {
				pkg = strings.TrimSuffix(filepath.Base(fdesc.GetName()), ".proto")
			}
			goPackage := "example.com/fake/" + strings.ReplaceAll(pkg, ".", "/")
			fdesc.Options.GoPackage = &goPackage
		}
	}
	gen, err := opts.New(req)
	if err != nil {
		return err
	}
	if err := generate(gen, conf); err != nil {
		gen.Error(err)
	}
	resp := gen.Response()
	out, err := proto.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := os.Stdout.Write(out); err != nil {
		return err
	}
	return nil
}

func generate(plugin *protogen.Plugin, conf generator.Configuration) error {
	// Enable "optional" keyword in front of type (e.g. optional string label = 1;)
	plugin.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
	if *conf.OutputMode == "source_relative" {
		for _, file := range plugin.Files {
			if !file.Generate {
				continue
			}
			outfileName := strings.TrimSuffix(file.Desc.Path(), filepath.Ext(file.Desc.Path())) + ".openapi.yaml"
			outputFile := plugin.NewGeneratedFile(outfileName, "")
			gen := generator.NewOpenAPIv3Generator(plugin, conf, []*protogen.File{file})
			if err := gen.Run(outputFile); err != nil {
				return err
			}
		}
	} else {
		outputFile := plugin.NewGeneratedFile("openapi.yaml", "")
		return generator.NewOpenAPIv3Generator(plugin, conf, plugin.Files).Run(outputFile)
	}
	return nil
}
