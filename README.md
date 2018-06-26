# Test Link Text Replacer
This tool search text in Summary, Preconditions, Step actions, Expected results in test cases with a specified search string. Then, replace directly searched strings to replace strings in Test Link DB.

**This tool modifies data of DB directly. Please backup current DB before to use.**

# Motivation

TestLink cannot replace strings which were searched by full-text search at the same time. if the server is replaced without DNS server, URL in test cases becomes dead-link. We must replace these dead-links one by one. It is painful.

# Usage

```
> tl_text_replacer.exe -db=<testlink database name> -project=<testlink project name> -search=<search string> -replace=<replace string>
```

This tool replace text only into the latest test case. 
