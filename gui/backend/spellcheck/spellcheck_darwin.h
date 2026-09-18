//go:build darwin && !ios

#ifndef SPELLCHECK_DARWIN_H
#define SPELLCHECK_DARWIN_H

#include <stdint.h>

// SpellRange represents a misspelled byte range.
typedef struct {
    int startByte;
    int lenBytes;
} SpellRange;

// SpellCheckResult holds an array of misspelled ranges.
typedef struct {
    SpellRange *ranges;
    int         count;
} SpellCheckResult;

// SuggestResult holds an array of suggestion strings.
typedef struct {
    char **suggestions;
    int    count;
} SuggestResult;

SpellCheckResult spellcheckCheck(const char *text, int textLen);
SuggestResult spellcheckSuggest(const char *text, int textLen,
    int startByte, int lenBytes);
void spellcheckLearn(const char *word, int wordLen);
void spellcheckFreeResult(SpellCheckResult r);
void spellcheckFreeSuggestResult(SuggestResult r);

#endif
