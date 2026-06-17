# Use case: comments and attachments

> Attachments require the **internal API** (session auth). See `setup-and-auth.md`.
> On the public `/api/v1` they may be unavailable and degrade with a message.

## Comments

```sh
mello comment add PROJ-12 -b "Shipping today."
echo "A longer comment" | mello comment add PROJ-12     # body via stdin
mello comment add PROJ-12 --body-file ./note.md
mello comment list PROJ-12
```

`mello ticket view PROJ-12` also shows the last comments inline.

## Attachments — one-shot

```sh
mello attachment list PROJ-12
mello attachment add PROJ-12 ./design.png ./spec.pdf    # upload now
mello attachment download PROJ-12 --dir ./downloads     # download all
```

`mello ticket view PROJ-12` lists attachments with sizes.

## Attachments — via the working set (batch with other edits)

Pull the ticket, drop files into its `attachments/` folder, and push:

```sh
mello pull PROJ-12
cp design.pdf .mello/boards/<board>/tickets/<ticket>/attachments/
mello status            # ~ PROJ-12  fields, 1 file(s)
mello push              # uploads design.pdf (POST /api/tickets/{id}/attachments)
```

New files under `attachments/` are detected by content hash and uploaded on the
next push; likewise new files under `comments/` are posted.
