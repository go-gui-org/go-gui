//go:build darwin && !ios

#import <AppKit/AppKit.h>
#include "spellcheck_darwin.h"
#include <stdlib.h>
#include <string.h>

SpellCheckResult spellcheckCheck(const char *text, int textLen) {
    @autoreleasepool {
        SpellCheckResult result = {0};
        if (text == NULL || textLen == 0) return result;

        NSString *str = [[NSString alloc]
            initWithBytes:text
            length:textLen
            encoding:NSUTF8StringEncoding];
        if (str == nil) return result;

        NSSpellChecker *checker = [NSSpellChecker sharedSpellChecker];
        NSInteger strLen = str.length;

        // Collect misspelled ranges.
        int capacity = 8;
        int count = 0;
        SpellRange *ranges = (SpellRange *)malloc(
            capacity * sizeof(SpellRange));
        if (ranges == NULL) return result;

        NSInteger offset = 0;
        NSInteger lastUTF16 = 0;
        int lastByte = 0;
        while (offset < strLen) {
            NSRange misspelled = [checker
                checkSpellingOfString:str
                startingAt:offset
                language:nil
                wrap:NO
                inSpellDocumentWithTag:0
                wordCount:NULL];
            if (misspelled.location == NSNotFound ||
                misspelled.length == 0) {
                break;
            }

            // Convert UTF-16 range to UTF-8 byte range
            // using incremental offset tracking.
            NSRange delta = NSMakeRange(lastUTF16,
                misspelled.location - lastUTF16);
            NSString *gap = [str substringWithRange:delta];
            lastByte += (int)[gap
                lengthOfBytesUsingEncoding:NSUTF8StringEncoding];
            int startByte = lastByte;

            NSString *word = [str substringWithRange:
                misspelled];
            int lenBytes = (int)[word
                lengthOfBytesUsingEncoding:NSUTF8StringEncoding];
            lastUTF16 = misspelled.location + misspelled.length;
            lastByte = startByte + lenBytes;

            if (count >= capacity) {
                capacity *= 2;
                // Via a temporary: a failed realloc returns NULL
                // and the original block stays valid, so assigning
                // back directly would leak it and NULL-deref below.
                SpellRange *grown = (SpellRange *)realloc(ranges,
                    capacity * sizeof(SpellRange));
                if (grown == NULL) {
                    free(ranges);
                    result.ranges = NULL;
                    result.count = 0;
                    return result;
                }
                ranges = grown;
            }
            ranges[count].startByte = startByte;
            ranges[count].lenBytes = lenBytes;
            count++;

            offset = misspelled.location + misspelled.length;
        }

        result.ranges = ranges;
        result.count = count;
        return result;
    }
}

SuggestResult spellcheckSuggest(const char *text, int textLen,
    int startByte, int lenBytes) {
    @autoreleasepool {
        SuggestResult result = {0};
        if (text == NULL || textLen == 0) return result;
        if (startByte < 0 || lenBytes < 0 ||
            startByte + lenBytes > textLen) return result;

        NSString *str = [[NSString alloc]
            initWithBytes:text
            length:textLen
            encoding:NSUTF8StringEncoding];
        if (str == nil) return result;

        // Convert byte offset to UTF-16 range.
        NSString *prefix = [[NSString alloc]
            initWithBytes:text
            length:startByte
            encoding:NSUTF8StringEncoding];
        NSString *word = [[NSString alloc]
            initWithBytes:text + startByte
            length:lenBytes
            encoding:NSUTF8StringEncoding];
        if (prefix == nil || word == nil) return result;

        NSRange utf16Range = NSMakeRange(prefix.length,
            word.length);

        NSSpellChecker *checker = [NSSpellChecker sharedSpellChecker];
        NSArray<NSString *> *guesses = [checker
            guessesForWordRange:utf16Range
            inString:str
            language:nil
            inSpellDocumentWithTag:0];

        if (guesses == nil || guesses.count == 0) return result;

        int count = (int)guesses.count;
        result.suggestions = (char **)malloc(
            count * sizeof(char *));
        if (result.suggestions == NULL) return result;
        result.count = count;
        for (int i = 0; i < count; i++) {
            const char *s = [guesses[i] UTF8String];
            char *dup = strdup(s);
            if (dup == NULL) {
                for (int j = 0; j < i; j++) {
                    free(result.suggestions[j]);
                }
                free(result.suggestions);
                result.suggestions = NULL;
                result.count = 0;
                return result;
            }
            result.suggestions[i] = dup;
        }
        return result;
    }
}

void spellcheckLearn(const char *word, int wordLen) {
    @autoreleasepool {
        if (word == NULL || wordLen <= 0) return;
        // Length-explicit, like Check/Suggest: a word with an
        // embedded NUL must not silently truncate at the NUL.
        NSString *str = [[NSString alloc]
            initWithBytes:word
            length:wordLen
            encoding:NSUTF8StringEncoding];
        if (str == nil) return;
        [[NSSpellChecker sharedSpellChecker] learnWord:str];
    }
}

void spellcheckFreeResult(SpellCheckResult r) {
    free(r.ranges);
}

void spellcheckFreeSuggestResult(SuggestResult r) {
    for (int i = 0; i < r.count; i++) {
        free(r.suggestions[i]);
    }
    free(r.suggestions);
}
