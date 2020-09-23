Conntrack Accounting
====================

This tool collects traffic information from conntrack and outputs them in a format suitable for Telegraf / InfluxDB or PostgreSQL.

Traffic is always accounted to the peer that opened the connection. Currently only IPv4 traffic is supported.

The collected traffic is exported in text format, which can either be read:
- by Telegraf and be stored in an InfluxDB time series database
- into a PostgreSQL database with our custom importer.

We used InfluxDB for saarCTF 2020, but it could hardly stand the load, our Grafana dashboards caused the database to freeze. 
The PostgreSQL has only been tested on historical data, not in a real-world situation. 


Installation
------------
- Download Go 1.15
- Run `go build` in directory conntrack_accounting_tool
- *(optional)* Run `go build` in directory conntrack_psql_insert


Usage
-----
Sample usage (from saarCTF) monitoring `10.32.0.0/11` and accounting traffic per `/24` subnet:

```
./conntrack_accounting -src=10.32.0.0/11 -src-group-mask=255.239.255.0 -dst=10.32.0.0/11 -dst-group-mask=255.239.255.0 -pipe=/tmp/conntrack_acct -output=/root/conntrack_data/new -track-open
./psql_insert -host=... -db=... -user=... -pass=... -watch=/root/conntrack_data/new -move=/root/conntrack_data/processed
```

The collected traffic statistics are imported into InfluxDB using [this Telegraf configuration](configs/telegraf_conntrack_acct.conf), which reads from `/tmp/conntrack_acct`. 
In parallel, collected traffic statistics are imported into a PostgreSQL database (in a table named `vpn_traffic`).
The generated text reports are preserved in `/root/conntrack_data/processed`, while the reports pending Postgres import are stored in `/root/conntrack_data/new`.
Finally a [Grafana](https://grafana.com/) instance is used to visualize traffic stats (using the provided dashboards).



Configurations
--------------
- Sample [Telegraf Plugin Configuration](configs/telegraf_conntrack_acct.conf) to collect results in InfluxDB
- Sample [Grafana Dashboards](configs/) to show results (PostgreSQL based)
- Sample systemd service file [for conntrack_accounting](configs/conntrack_accounting.service) / [for postgresql importer](configs/conntrack_psql_insert.service)


About
-----
Conntrack accounting was developed by Markus Bauer as part of the saarCTF 2020 game infrastructure, but is usable independent of other parts. 
