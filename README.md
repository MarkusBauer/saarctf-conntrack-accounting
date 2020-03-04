Conntrack Accounting
====================

This tool collects traffic information from conntrack and outputs them in a format suitable for Telegraf / InfluxDB.

Traffic is always accounted to the peer that opened the connection.


Installation
------------
- Download Go 1.14
- Run `go build` in this directory


Usage
-----
Sample usage (from saarCTF) monitoring `10.32.0.0/11` and accounting traffic per `/24` subnet:

`./conntrack_accounting -src=10.32.0.0/11 -src-group-mask=255.239.255.0 -dst=10.32.0.0/11 -dst-group-mask=255.239.255.0 -pipe=/tmp/conntrack_acct -track-open`


Telegraf configuration
----------------------
```
[[inputs.tail]]
  files = ["/tmp/conntrack_acct"]
  from_beginning = false
  pipe = true
  data_format = "csv"
  csv_header_row_count = 0
  csv_column_names = ["proto", "src", "dst", "port", "src_packets", "dst_packets", "src_bytes", "dst_bytes", "connection_count", "connection_times", "open_connections"]
  csv_column_types = ["string", "string", "string", "int", "int", "int", "int", "int", "int", "int", "int"]
  csv_tag_columns = ["proto", "src", "dst", "port"]
  csv_skip_rows = 0
  csv_skip_columns = 0
  name_override="traffic"
```