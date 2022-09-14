package main

import (
	"dagger.io/dagger"
	"dagger.io/dagger/core"
)

dagger.#Plan & {
	actions: {
		pull: core.#Pull & {
			source: "alpine"
		}
		pull315: core.#Pull & {
			source: "alpine:3.15"
		}
		exec: core.#Exec & {
			input: pull.output
			args: ["echo", "hello world", "from joel 3"]
		}

		print_versions: {
			latest: core.#Exec & {
				input: pull.output
				args: ["cat", "/etc/alpine-release"]
			}
			"315": core.#Exec & {
				input: pull315.output
				args: ["cat", "/etc/alpine-release"]
			}
		}
		exec3: core.#Exec & {
			input: pull315.output
			args: ["cat", "/etc/alpine-release"]
		}
		exec4: core.#Exec & {
			input: pull.output
			args: ["cat", "/315/etc/alpine-release"]
			mounts: "315": {
				dest:     "/315"
				contents: pull315.output
			}
		}
		exec5: core.#Exec & {
			input: pull.output
			args: ["cat", "/etc/alpine-release"]
			mounts: "315": {
				dest:     "/315"
				contents: pull315.output
			}
		}

		exec2: core.#Exec & {
			input: pull.output
			env: "JOEL": "joel1234"
			args: ["printenv", "JOEL"]
		}
	}
}
