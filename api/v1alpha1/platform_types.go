/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

const (
	// All the different types of hooks.

	// GITHUB based hooks
	GITHUB = iota + 1
	// GITLAB based hooks
	GITLAB
	// GITEA based hooks
	GITEA
)

// Platform defines a platform like Github, Gitlab etc.
type Platform struct {
	// ID of the platform. This is chosen.
	ID int `json:"id"`
	// Name of the platform.
	Name string `json:"name"`
}

// SupportedPlatforms a map of supported platforms by Krok.
var SupportedPlatforms = map[int]Platform{
	GITHUB: {
		ID:   GITHUB,
		Name: "github",
	},
	GITLAB: {
		ID:   GITLAB,
		Name: "gitlab",
	},
	GITEA: {
		ID:   GITEA,
		Name: "gitea",
	},
}
