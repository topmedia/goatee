# goatee

Fetch PDF email attachments via IMAP

## Building and running

~~~~~
go get
go build
./goatee
~~~~~

## Sample configuration

~~~~~
# goatee.cfg
Server = "imap.gmail.com"
User = "user@domain.com"
Password = "foobarbaz"
Destination = "pdfs"
~~~~~

## Caveats

* Only works with IMAPS
* Only works with PDF attachments
* Destination directory is relative to the working directory
