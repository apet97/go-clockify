Find clients on a workspace
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

query Parameters
name	
string
Default: "##default"
Example: name=Client X
Filters client results that matches with the string provided in their client name.

sort-column	
string
Default: "NAME"
Example: sort-column=NAME
Column name that will be used as criteria for sorting results.

sort-order	
string
Default: "##default"
Example: sort-order=ASCENDING
Sorting mode

page	
integer <int32>
Default: 1
Example: page=1
Page number.

page-size	
integer <int32> >= 1
Default: 50
Example: page-size=50
Page size.

archived	
string
Default: "##default"
Filter whether to include archived clients or not.

Responses
200 OK
Response Schema: application/json
Array 
address	
string
Default: "##default"
Represents client's address.

archived	
boolean
Default: false
Represents whether a client is archived or not.

ccEmails	
Array of strings
Default: "##default"
Represents additional emails for sending invoices.

currencyCode	
string
Default: "##default"
Represents client currency code.

currencyId	
string
Default: "##default"
Represents currency identifier across the system.

email	
string
Default: "##default"
Represents client email.

id	
string
Default: "##default"
Represents client identifier across the system.

name	
string
Default: "##default"
Represents client name.

note	
string
Default: "##default"
Represents saved notes for the client.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/clients
https://api.clockify.me/api/v1/workspaces/{workspaceId}/clients
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"address": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
"archived": false,
"ccEmails": "clientx@example.com",
"currencyCode": "USD",
"currencyId": "33t687e29ae1f428e7ebe505",
"email": "clientx@example.com",
"id": "44a687e29ae1f428e7ebe305",
"name": "Client X",
"note": "This is a sample note for the client.",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Add a new client
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Request Body schema: application/json
required
address	
string [ 0 .. 3000 ] characters
Default: "##default"
Represents a client's address.

email	
string <email>
Default: "##default"
Represents a client email.

name	
string [ 0 .. 100 ] characters
Default: "##default"
Represents a client name.

note	
string [ 0 .. 3000 ] characters
Default: "##default"
Represents additional notes for the client.

Responses
201 Created
Response Schema: application/json
address	
string
Default: "##default"
Represents client's address.

archived	
boolean
Default: false
Represents whether a client is archived or not.

ccEmails	
Array of strings
Default: "##default"
Represents additional emails for sending invoices.

currencyCode	
string
Default: "##default"
Represents client currency code.

currencyId	
string
Default: "##default"
Represents currency identifier across the system.

email	
string
Default: "##default"
Represents client email.

id	
string
Default: "##default"
Represents client identifier across the system.

name	
string
Default: "##default"
Represents client name.

note	
string
Default: "##default"
Represents saved notes for the client.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/clients
https://api.clockify.me/api/v1/workspaces/{workspaceId}/clients
Request samples
Payload
Content type
application/json

Copy
{
"address": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
"email": "clientx@example.com",
"name": "Client X",
"note": "This is a sample note for the client."
}
Response samples
201
Content type
application/json

Copy
{
"address": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
"archived": false,
"ccEmails": "clientx@example.com",
"currencyCode": "USD",
"currencyId": "33t687e29ae1f428e7ebe505",
"email": "clientx@example.com",
"id": "44a687e29ae1f428e7ebe305",
"name": "Client X",
"note": "This is a sample note for the client.",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Delete a client
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
id
required
string
Default: "##default"
Example: 44a687e29ae1f428e7ebe305
Represents a client identifier across the system.

workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Responses
200 OK
Response Schema: application/json
address	
string
Default: "##default"
Represents client's address.

archived	
boolean
Default: false
Represents whether a client is archived or not.

ccEmails	
Array of strings
Default: "##default"
Represents additional emails for sending invoices.

currencyId	
string
Default: "##default"
Represents currency identifier across the system.

email	
string
Default: "##default"
Represents client email.

id	
string
Default: "##default"
Represents client identifier across the system.

name	
string
Default: "##default"
Represents client name.

note	
string
Default: "##default"
Represents saved notes for the client.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


delete
/v1/workspaces/{workspaceId}/clients/{id}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/clients/{id}
Response samples
200
Content type
application/json

Copy
{
"address": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
"archived": false,
"ccEmails": "clientx@example.com",
"currencyId": "33t687e29ae1f428e7ebe505",
"email": "clientx@example.com",
"id": "44a687e29ae1f428e7ebe305",
"name": "Client X",
"note": "This is a sample note for the client.",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Get a client by ID
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
id
required
string
Default: "##default"
Example: 44a687e29ae1f428e7ebe305
Represents a client identifier across the system.

workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Responses
200 OK
Response Schema: application/json
address	
string
Default: "##default"
Represents client's address.

archived	
boolean
Default: false
Represents whether a client is archived or not.

ccEmails	
Array of strings
Default: "##default"
Represents additional emails for sending invoices.

currencyCode	
string
Default: "##default"
Represents client currency code.

currencyId	
string
Default: "##default"
Represents currency identifier across the system.

email	
string
Default: "##default"
Represents client email.

id	
string
Default: "##default"
Represents client identifier across the system.

name	
string
Default: "##default"
Represents client name.

note	
string
Default: "##default"
Represents saved notes for the client.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/clients/{id}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/clients/{id}
Response samples
200
Content type
application/json

Copy
{
"address": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
"archived": false,
"ccEmails": "clientx@example.com",
"currencyCode": "USD",
"currencyId": "33t687e29ae1f428e7ebe505",
"email": "clientx@example.com",
"id": "44a687e29ae1f428e7ebe305",
"name": "Client X",
"note": "This is a sample note for the client.",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Update a client
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
id
required
string
Default: "##default"
Example: 44a687e29ae1f428e7ebe305
Represents a client identifier across the system.

workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

query Parameters
archive-projects	
boolean
mark-tasks-as-done	
boolean
Request Body schema: application/json
required
address	
string [ 0 .. 3000 ] characters
Default: "##default"
Represents a client's address.

archived	
boolean
Default: false
Indicates if client will be archived or not.

ccEmails	
Array of strings <email> [ 0 .. 3 ] items [ items <email > ]
currencyId	
string
Default: "##default"
Represents a currency identifier across the system.

email	
string <email>
Default: "##default"
Represents a client email.

name	
string [ 0 .. 100 ] characters
Default: "##default"
Represents a client name.

note	
string [ 0 .. 3000 ] characters
Default: "##default"
Represents additional notes for the client.

Responses
200 OK
Response Schema: application/json
address	
string
Default: "##default"
Represents client's address.

archived	
boolean
Default: false
Represents whether a client is archived or not.

ccEmails	
Array of strings
Default: "##default"
Represents additional emails for sending invoices.

currencyId	
string
Default: "##default"
Represents currency identifier across the system.

email	
string
Default: "##default"
Represents client email.

id	
string
Default: "##default"
Represents client identifier across the system.

name	
string
Default: "##default"
Represents client name.

note	
string
Default: "##default"
Represents saved notes for the client.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


put
/v1/workspaces/{workspaceId}/clients/{id}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/clients/{id}
Request samples
Payload
Content type
application/json

Copy
Expand allCollapse all
{
"address": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
"archived": false,
"ccEmails": [
"user@example.com"
],
"currencyId": "53a687e29ae1f428e7ebe888",
"email": "clientx@example.com",
"name": "Client X",
"note": "This is a sample note for the client."
}
Response samples
200
Content type
application/json

Copy
{
"address": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
"archived": false,
"ccEmails": "clientx@example.com",
"currencyId": "33t687e29ae1f428e7ebe505",
"email": "clientx@example.com",
"id": "44a687e29ae1f428e7ebe305",
"name": "Client X",
"note": "This is a sample note for the client.",
"workspaceId": "64a687e29ae1f428e7ebe303"
}