Find tags on a workspace
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
Example: name=feature_X
If provided, you'll get a filtered list of tags that matches the provided string in their name.

strict-name-search	
boolean
Default: false
Flag to toggle on/off strict search mode. When set to true, search by name will only return tags whose name exactly matches the string value given for the 'name' parameter. When set to false, results will also include tags whose name contain the string value, but could be longer than the string value itself. For example, if there is a tag with the name 'applications', and the search value is 'app', setting strict-name-search to true will not return that tag in the results, whereas setting it to false will.

excluded-ids	
string
Example: excluded-ids=90p687e29ae1f428e7ebe657&excluded-ids=3r8687e29ae1f428e7eg567y
Represents a list of excluded ids

sort-column	
string
Enum: "ID" "NAME"
Example: sort-column=NAME
Represents a column to be used as sorting criteria.

sort-order	
string
Enum: "ASCENDING" "DESCENDING"
Example: sort-order=ASCENDING
Represents a sorting mode.

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
boolean
Default: false
Example: archived=false
Filters the result whether tags are archived or not.

Responses
200 OK
Response Schema: application/json
Array 
archived	
boolean
Default: false
Indicates whether a tag is archived or not.

id	
string
Default: "##default"
Represents tag identifier across the system.

name	
string
Default: "##default"
Represents tag name.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/tags
https://api.clockify.me/api/v1/workspaces/{workspaceId}/tags
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"archived": false,
"id": "21s687e29ae1f428e7ebe404",
"name": "Sprint1",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Add a new tag
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
name	
string [ 0 .. 100 ] characters
Default: "##default"
Represents a tag name.

Responses
201 Created
Response Schema: application/json
archived	
boolean
Default: false
Indicates whether a tag is archived or not.

id	
string
Default: "##default"
Represents tag identifier across the system.

name	
string
Default: "##default"
Represents tag name.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/tags
https://api.clockify.me/api/v1/workspaces/{workspaceId}/tags
Request samples
Payload
Content type
application/json

Copy
{
"name": "Sprint1"
}
Response samples
201
Content type
application/json

Copy
{
"archived": false,
"id": "21s687e29ae1f428e7ebe404",
"name": "Sprint1",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Delete a tag
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
id
required
string
Default: "##default"
Example: 21s687e29ae1f428e7ebe404
Represents a tag identifier across the system.

workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Responses
200 OK
Response Schema: application/json
archived	
boolean
Default: false
Indicates whether a tag is archived or not.

id	
string
Default: "##default"
Represents tag identifier across the system.

name	
string
Default: "##default"
Represents tag name.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


delete
/v1/workspaces/{workspaceId}/tags/{id}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/tags/{id}
Response samples
200
Content type
application/json

Copy
{
"archived": false,
"id": "21s687e29ae1f428e7ebe404",
"name": "Sprint1",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Get a tag by ID
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
id
required
string
Default: "##default"
Example: 21s687e29ae1f428e7ebe404
Represents a tag identifier across the system.

workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Responses
200 OK
Response Schema: application/json
archived	
boolean
Default: false
Indicates whether a tag is archived or not.

id	
string
Default: "##default"
Represents tag identifier across the system.

name	
string
Default: "##default"
Represents tag name.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/tags/{id}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/tags/{id}
Response samples
200
Content type
application/json

Copy
{
"archived": false,
"id": "21s687e29ae1f428e7ebe404",
"name": "Sprint1",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Update a tag
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
id
required
string
Default: "##default"
Example: 21s687e29ae1f428e7ebe404
Represents a tag identifier across the system.

workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Request Body schema: application/json
required
archived	
boolean
Default: false
Indicates whether a tag will be archived or not.

name	
string [ 0 .. 100 ] characters
Default: "##default"
Represents a tag name.

Responses
200 OK
Response Schema: application/json
archived	
boolean
Default: false
Indicates whether a tag is archived or not.

id	
string
Default: "##default"
Represents tag identifier across the system.

name	
string
Default: "##default"
Represents tag name.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


put
/v1/workspaces/{workspaceId}/tags/{id}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/tags/{id}
Request samples
Payload
Content type
application/json

Copy
{
"archived": false,
"name": "Sprint1"
}
Response samples
200
Content type
application/json

Copy
{
"archived": false,
"id": "21s687e29ae1f428e7ebe404",
"name": "Sprint1",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
