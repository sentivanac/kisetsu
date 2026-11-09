package commons

// Wikimedia Commons has no safe-for-work flag. Kisetsu reduces the risk in
// three ways (see ADR 0011):
//
//  1. searches are limited to the curated Quality and Featured collections;
//  2. serverExcludes are sent as "-term" with every search;
//  3. results whose title, description or categories contain a risky word
//     are dropped locally (providers.LooksUnsafe).
//
// This is a best-effort filter, not a guarantee, which is why the source is
// off by default and the UI says so.

// serverExcludes is kept short: Commons limits a search to 300 characters.
var serverExcludes = []string{"nude", "naked", "nudity", "erotic"}
