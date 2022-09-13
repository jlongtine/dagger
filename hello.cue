package main

import (
	"dagger.io/dagger"
	"dagger.io/dagger/core"
)

dagger.#Plan & {
	actions: {
		pull: core.#Pull & {
			source: "index.docker.io/alpine"
		}
		exec: core.#Exec & {
			input: pull.output
			args: ["echo", "hello world", "from joel"]
		}
		exec2: core.#Exec & {
			input: pull.output
			env: "JOEL": "joel"
			args: ["printenv", "JOEL"]
		}
		exec3: core.#Exec & {
			input: pull.output
			args: ["exit", "20"]
		}
	}
}
