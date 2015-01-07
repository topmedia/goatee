# goatee

Fetch PDF email attachments via IMAP

## Building and running

~~~~~
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
* There is no concept of remembering which mails have been
  processed (yet)
* It needs more error checking
