Conntrack Accounting
====================

This tool collects traffic information from conntrack and outputs them in a format suitable for Telegraf / InfluxDB.

Traffic is always accounted to the peer that opened the connection.


Installation
------------
- Download Go 1.14
- Run `go build` in directory conntrack_accounting_tool


Usage
-----
Sample usage (from saarCTF) monitoring `10.32.0.0/11` and accounting traffic per `/24` subnet:

`./conntrack_accounting -src=10.32.0.0/11 -src-group-mask=255.239.255.0 -dst=10.32.0.0/11 -dst-group-mask=255.239.255.0 -pipe=/tmp/conntrack_acct -track-open`


Configurations
--------------
- Sample [Telegraf Plugin Configuration](configs/telegraf_conntrack_acct.conf) to collect results in InfluxDB
- Sample [Grafana Dashboards](configs/) to show results (PostgreSQL based)
- Sample [systemd service file](configs/conntrack_accounting.service)

