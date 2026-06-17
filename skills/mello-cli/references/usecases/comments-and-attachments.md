# Use case: comments and attachments

Comments:

```sh
mello comment add PROJ-12 -b "Shipping today."
echo "A longer comment" | mello comment add PROJ-12     # body via stdin
mello comment add PROJ-12 --body-file ./note.md
mello comment list PROJ-12
mello comment list PROJ-12 --json
```

Attachments:

```sh
mello attachment add PROJ-12 ./design.png ./spec.pdf    # upload one or more
mello attachment list PROJ-12
mello attachment download PROJ-12 --dir ./downloads
```

Attachments are an optional endpoint; if the instance doesn't support them the
command prints a clear "not supported" message and exits non-zero.
