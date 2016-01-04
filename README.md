# Suger

`suger` is a tool to crawl and scrape the film classification database of Singapore's Media Development Authority (MDA).

## About

`suger` was a quick and dirty effort (one evening's work). I was learning Go as I went, so `suger` has some unfortunate warts. The few things that new Go users may find interesting in this package:

 - the lightweight implementation of subcommands, including use of the `flag` package,
 - the use of channels for 'job control', and 
 - implementing most of the application in `package libsuger`, which is imported by `package main`.

The MDA classification database is an ASP application, and I have zero experience with ASP, but I was able to reverse engineer ASP requests for my limited needs. Since there is little information about crawling ASP applications online, I plan to document my solution in the future.

In the unlikely event that someone out there actually wants to look at the classification database as a dataset, don't bother actually crawling the database, which is a slow process (72,000 titles). Just expand the tarball `html.tgz` and modify the `suger scrape` subcommand for your purposes (or use your own tool to scrape). (The html directory is not tracked. Too many small files. Thus the tarball.)

## Usage

Output of `$ suger`:

```
Usage of suger:
    suger crawl [flags]
        crawl classification database
    suger scrape [flags]
        scrape downloaded html files
(Use the -h flag for help with each subcommand.)
```

Output of `$ suger crawl -h`:

```
Usage of crawl:
  -count int
        crawl this many results (default 25)
  -html string
        directory to write HTML files (default "out/html")
  -start int
        start at this result (default 1)
  -workers int
        number of workers (default 1)
```

Output of `$ suger scrape -h`

```
Usage of scrape:
  -html string
        directory to read HTML files (default "out/html")
  -out string
        directory for output (default "out")
```


