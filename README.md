# thirteen
> Primitive MySQL Monitor
>


[![Build Status](https://travis-ci.org/murdinc/thirteen.svg)](https://travis-ci.org/murdinc/thirteen)

## Intro
**thirteen** is a primitive (for now) Terminal UI MySQL monitor.

## Installation
To install awsm, simply copy/paste the following command into your terminal:
```
curl -s http://dl.sudoba.sh/get/thirteen | sh
```


## Configuration
**thirteen** uses the same AWS configuration as [awsm](https://github.com/murdinc/awsm) (which is the same configuration file as the AWS SDK `~/.aws/credentials`). It also loads a separate configuration from `~/.thirteen` which contains the DB Connection information, as well as a query for gathering the "Item Count" value.

Example `~/.thirteen` config:

```
DB = "db_name"
PORT = "3306"
USER = "db_user"
PASSWORD = "db_password"
ITEMCOUNTQUERY = "SELECT COUNT(*) FROM item_table"

```

