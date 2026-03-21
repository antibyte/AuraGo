# Address Book Tool Manual

## Tool Name: `address_book`

## Purpose
Manage the user's personal address book / contacts database. You can search, list, add, update, and delete contacts.

## Operations

### `list`
List all contacts in the address book.
- No parameters required.
- Returns all contacts sorted by name.

### `search`
Search contacts by name, email, phone, mobile, or relationship.
- **query** (required): Search term to match across contact fields.

### `add`
Add a new contact to the address book.
- **name** (required): Full name of the contact.
- **email** (optional): Email address.
- **phone** (optional): Phone number.
- **mobile** (optional): Mobile phone number.
- **address** (optional): Postal address.
- **relationship** (optional): Relationship type (e.g., friend, colleague, family, client).

### `update`
Update an existing contact. Only provided fields will be changed.
- **id** (required): Contact ID (from list or search results).
- **name** (optional): Updated name.
- **email** (optional): Updated email.
- **phone** (optional): Updated phone.
- **mobile** (optional): Updated mobile.
- **address** (optional): Updated postal address.
- **relationship** (optional): Updated relationship.

### `delete`
Delete a contact from the address book.
- **id** (required): Contact ID to delete.

## Notes
- Contact data is stored locally in SQLite and never sent to external services.
- The user can also manage contacts via the Knowledge Center in the Web UI.
- Always confirm with the user before deleting contacts.
