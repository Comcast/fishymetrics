/*
 * Copyright 2023 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package buildinfo

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"text/tabwriter"
)

const Unknown = "unknown"

var (
	gitVersion  = Unknown
	gitRevision = Unknown
	date        = Unknown

	Info info
)

type info struct {
	Arch         string `json:"arch"`
	Compiler     string `json:"compiler"`
	Date         string `json:"build_date"`
	GitRevision  string `json:"revision"`
	GitVersion   string `json:"version"`
	GoVersion    string `json:"go_version"`
	OS           string `json:"os"`
	RaceDetector bool   `json:"race_detector"`
}

func init() {
	Info.Arch = runtime.GOARCH
	Info.Compiler = runtime.Compiler
	Info.Date = date
	Info.GitRevision = gitRevision
	Info.GitVersion = gitVersion
	Info.GoVersion = runtime.Version()
	Info.OS = runtime.GOOS
}

func Print(dest io.Writer) error {
	w := tabwriter.NewWriter(dest, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Build Date:\t%q\n", Info.Date)
	fmt.Fprintf(w, "Go ARCH:\t%q\n", Info.Arch)
	fmt.Fprintf(w, "Go Compiler:\t%q\n", Info.Compiler)
	fmt.Fprintf(w, "Go OS:\t%q\n", Info.OS)
	fmt.Fprintf(w, "Go Version:\t%q\n", Info.GoVersion)
	fmt.Fprintf(w, "Revision:\t%q\n", Info.GitRevision)
	fmt.Fprintf(w, "Race Detector:\t%v\n", Info.RaceDetector)
	fmt.Fprintf(w, "Version:\t%q\n", Info.GitVersion)
	return w.Flush()
}

func JSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	return enc.Encode(Info)
}
