Find all groups on a workspace
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
project-id	
string
Example: project-id=5a0ab5acb07987125438b60f
If provided, you'll get a filtered list of groups that matches the string provided in their project id.

name	
string
Default: "##default"
Example: name=development_team
If provided, you'll get a filtered list of groups that matches the string provided in their name.

sort-column	
string
Enum: "ID" "NAME"
Example: sort-column=NAME
Column to be used as the sorting criteria.

sort-order	
string
Enum: "ASCENDING" "DESCENDING"
Example: sort-order=ASCENDING
Sorting mode.

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

includeTeamManagers	
boolean
Default: false
Example: includeTeamManagers=true
If provided, you'll get a list of team managers assigned to this user group.

Responses
200 OK
Response Schema: application/json
Array 
id	
string
Default: "##default"
Represents a user group identifier across the system.

name	
string
Default: "##default"
Represents a user group name.

teamManagers	
Array of objects (UserRedactedDtoV1)
Default: "##default"
Represents a list of assigned team managers for this user group.

Array 
id	
string
name	
string
userIds	
Array of strings
Default: "##default"
Represents a list of users' identifiers across the system.

workspaceId	
string
Default: "##default"
Represents a workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/user-groups
https://api.clockify.me/api/v1/workspaces/{workspaceId}/user-groups
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"id": "76a687e29ae1f428e7ebe101",
"name": "development_team",
"teamManagers": [
{
"id": "672323eb0024343a1585e8a7",
"name": "Jane Doe"
}
],
"userIds": [
"5a0ab5acb07987125438b60f",
"98j4b5acb07987125437y32"
],
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Add a new group
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
Represents a user group name.

Responses
201 Created
Response Schema: application/json
id	
string
Default: "##default"
Represents a user group identifier across the system.

name	
string
Default: "##default"
Represents a user group name.

teamManagers	
Array of objects (UserRedactedDtoV1)
Default: "##default"
Represents a list of assigned team managers for this user group.

Array 
id	
string
name	
string
userIds	
Array of strings
Default: "##default"
Represents a list of users' identifiers across the system.

workspaceId	
string
Default: "##default"
Represents a workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/user-groups
https://api.clockify.me/api/v1/workspaces/{workspaceId}/user-groups
Request samples
Payload
Content type
application/json

Copy
{
"name": "development_team"
}
Response samples
201
Content type
application/json

Copy
Expand allCollapse all
{
"id": "76a687e29ae1f428e7ebe101",
"name": "development_team",
"teamManagers": [
{
"id": "672323eb0024343a1585e8a7",
"name": "Jane Doe"
}
],
"userIds": [
"5a0ab5acb07987125438b60f",
"98j4b5acb07987125437y32"
],
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Delete a group
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

id
required
string
Default: "##default"
Example: 76a687e29ae1f428e7ebe101
Represents a user group identifier across the system.

Responses
200 OK
Response Schema: application/json
id	
string
Default: "##default"
Represents a user group identifier across the system.

name	
string
Default: "##default"
Represents a user group name.

teamManagers	
Array of objects (UserRedactedDtoV1)
Default: "##default"
Represents a list of assigned team managers for this user group.

Array 
id	
string
name	
string
userIds	
Array of strings
Default: "##default"
Represents a list of users' identifiers across the system.

workspaceId	
string
Default: "##default"
Represents a workspace identifier across the system.


delete
/v1/workspaces/{workspaceId}/user-groups/{id}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/user-groups/{id}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"id": "76a687e29ae1f428e7ebe101",
"name": "development_team",
"teamManagers": [
{
"id": "672323eb0024343a1585e8a7",
"name": "Jane Doe"
}
],
"userIds": [
"5a0ab5acb07987125438b60f",
"98j4b5acb07987125437y32"
],
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Update a group
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
id
required
string
Default: "##default"
Example: 76a687e29ae1f428e7ebe101
Represents a user group identifier across the system.

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
Represents a user group name.

Responses
200 OK
Response Schema: application/json
id	
string
Default: "##default"
Represents a user group identifier across the system.

name	
string
Default: "##default"
Represents a user group name.

teamManagers	
Array of objects (UserRedactedDtoV1)
Default: "##default"
Represents a list of assigned team managers for this user group.

Array 
id	
string
name	
string
userIds	
Array of strings
Default: "##default"
Represents a list of users' identifiers across the system.

workspaceId	
string
Default: "##default"
Represents a workspace identifier across the system.


put
/v1/workspaces/{workspaceId}/user-groups/{id}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/user-groups/{id}
Request samples
Payload
Content type
application/json

Copy
{
"name": "development_team"
}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"id": "76a687e29ae1f428e7ebe101",
"name": "development_team",
"teamManagers": [
{
"id": "672323eb0024343a1585e8a7",
"name": "Jane Doe"
}
],
"userIds": [
"5a0ab5acb07987125438b60f",
"98j4b5acb07987125437y32"
],
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Add users to a group
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

userGroupId
required
string
Default: "##default"
Example: 76a687e29ae1f428e7ebe101
Represents a user group identifier across the system.

Request Body schema: application/json
required
userId
required
string
Default: "##default"
Represents a user identifier across the system.

Responses
200 OK
Response Schema: application/json
id	
string
Default: "##default"
Represents a user group identifier across the system.

name	
string
Default: "##default"
Represents a user group name.

teamManagers	
Array of objects (UserRedactedDtoV1)
Default: "##default"
Represents a list of assigned team managers for this user group.

Array 
id	
string
name	
string
userIds	
Array of strings
Default: "##default"
Represents a list of users' identifiers across the system.

workspaceId	
string
Default: "##default"
Represents a workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/user-groups/{userGroupId}/users
https://api.clockify.me/api/v1/workspaces/{workspaceId}/user-groups/{userGroupId}/users
Request samples
Payload
Content type
application/json

Copy
"##default"
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"id": "76a687e29ae1f428e7ebe101",
"name": "development_team",
"teamManagers": [
{
"id": "672323eb0024343a1585e8a7",
"name": "Jane Doe"
}
],
"userIds": [
"5a0ab5acb07987125438b60f",
"98j4b5acb07987125437y32"
],
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Remove a user from a group
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

userGroupId
required
string
Default: "##default"
Example: 76a687e29ae1f428e7ebe101
Represents a user group identifier across the system.

userId
required
string
Default: "##default"
Example: 5a0ab5acb07987125438b60f
Represents a user identifier across the system.

Responses
200 OK
Response Schema: application/json
id	
string
Default: "##default"
Represents a user group identifier across the system.

name	
string
Default: "##default"
Represents a user group name.

teamManagers	
Array of objects (UserRedactedDtoV1)
Default: "##default"
Represents a list of assigned team managers for this user group.

Array 
id	
string
name	
string
userIds	
Array of strings
Default: "##default"
Represents a list of users' identifiers across the system.

workspaceId	
string
Default: "##default"
Represents a workspace identifier across the system.


delete
/v1/workspaces/{workspaceId}/user-groups/{userGroupId}/users/{userId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/user-groups/{userGroupId}/users/{userId}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"id": "76a687e29ae1f428e7ebe101",
"name": "development_team",
"teamManagers": [
{
"id": "672323eb0024343a1585e8a7",
"name": "Jane Doe"
}
],
"userIds": [
"5a0ab5acb07987125438b60f",
"98j4b5acb07987125437y32"
],
"workspaceId": "64a687e29ae1f428e7ebe303"
}
