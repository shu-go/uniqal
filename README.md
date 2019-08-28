# uniqal

make each event be unique

## Options:
```
  -s, --start DATE             defaults to today
  -n, --items NUMBER           the number of events from --start (default: 10)
  -k, --keys LIST_OF_STRINGS   comman-separated keys to test uniquity of events (default: Description,Summary,Start,End)
  --id, --calendar-id           (default: primary)
  -c, --credentials FILE_NAME  your client configuration file from Google Developer Console (default: ./credentials.json)
  -t, --token FILE_NAME        file path to read/write retrieved token (default: ./token.json)
  --auth-port NUMBER            (default: 7878)
  --dry, --dry-run             do not exec
```

## Usage:
```
  uniqal --credential=./my_credentials.json --items=100 --start=2019-08-30
```

## how to start

1. go to https://console.cloud.google.com
2. make a new project
3. enable Google Calendar API from Library
4. download credential json
5. rename it as credentials.json and place it in current working dir

## --keys option

* created
* description
* end
* etag
* hangoutLink
* htmllink
* icaluid
* id
* location
* start
* summary
* updated

--keys=summary,start,end  may match for your needs.
And then, --dry is useful for testing.
