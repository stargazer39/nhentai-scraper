package main

import "regexp"

const HOME = "https://nhentai.to/"
const NOT_FOUND = "ntfound1"

var FIND_DATA_REGEX = regexp.MustCompile(`N\.reader\([\s\S]*?(}\);)`)
