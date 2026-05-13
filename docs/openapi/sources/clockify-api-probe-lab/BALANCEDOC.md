Balance
Get balances for a policy
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

policyId
required
string
Default: "##default"
Example: 63034cd0cb0fb876a57e93ad
Represents a policy identifier across the system.

query Parameters
page	
integer <int32> <= 1000
Default: 1
Example: page=1
page-size	
integer <int32> [ 1 .. 200 ]
Default: 50
Example: page-size=50
sort	
string
Enum: "USER" "POLICY" "USED" "BALANCE" "TOTAL"
Example: sort=USER
If provided, you'll get result sorted by sort column.

sort-order	
string
Enum: "ASCENDING" "DESCENDING"
Example: sort-order=ASCENDING
Sort results in ascending or descending order.

Responses
200 OK
Response Schema: */*
balances	
Array of objects (BalanceDtoV1)
Default: "##default"
Array 
balance	
number <double>
Represents the balance amount of the time unit

id	
string
Default: "##default"
Represent balance identifier across the system.

negativeBalanceAmount	
number <double>
Represent negative balance amount.

negativeBalanceLimit	
boolean
Default: false
Indicates whether the negative balance limit is allowed.

policyArchived	
boolean
Default: false
Indicates whether the policy is archived.

policyId	
string
Default: "##default"
Represent policy identifier across the system.

policyName	
string
Default: "##default"
Represent policy name.

policyTimeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represent policy time unit.

total	
number <double>
Represents the total amount

used	
number <double>
Represents the balance used amount

userId	
string
Default: "##default"
Represent user identifier across the system.

userName	
string
Default: "##default"
Represent user's username.

workspaceId	
string
Default: "##default"
Represent workspace identifier across the system.

count	
integer <int32>
Represents the count of balances.


get
/v1/workspaces/{workspaceId}/time-off/balance/policy/{policyId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-off/balance/policy/{policyId}
Update a balance
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

policyId
required
string
Default: "##default"
Example: 63034cd0cb0fb876a57e93ad
Represents a policy identifier across the system.

Request Body schema: application/json
required
note	
string
Default: "##default"
Represents a new balance note value.

userIds
required
Array of strings non-empty unique
Default: "##default"
Represents the list of users' identifiers whose balance is to be updated.

value
required
number <double> [ -10000 .. 10000 ]
Represents a new balance value.

Responses
204 No Content

patch
/v1/workspaces/{workspaceId}/time-off/balance/policy/{policyId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-off/balance/policy/{policyId}
Request samples
Payload
Content type
application/json

Copy
Expand allCollapse all
{
"note": "Bonus days added.",
"userIds": [
"5b715448b079875110792222",
"5b715448b079875110791111"
],
"value": 22
}
Get balance for a user
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

userId
required
string
Default: "##default"
Example: 60f924bafdaf031696ec6218
Represents a user identifier across the system.

query Parameters
page	
string <= 1000
Default: "##default"
Page number.

page-size	
string [ 1 .. 200 ]
Default: "##default"
Page size.

sort	
string
Enum: "USER" "POLICY" "USED" "BALANCE" "TOTAL"
Example: sort=POLICY
Sort result based on given criteria

sort-order	
string
Enum: "ASCENDING" "DESCENDING"
Example: sort-order=ASCENDING
Sort result by providing sort order.

Responses
200 OK
Response Schema: */*
balances	
Array of objects (BalanceDtoV1)
Default: "##default"
Array 
balance	
number <double>
Represents the balance amount of the time unit

id	
string
Default: "##default"
Represent balance identifier across the system.

negativeBalanceAmount	
number <double>
Represent negative balance amount.

negativeBalanceLimit	
boolean
Default: false
Indicates whether the negative balance limit is allowed.

policyArchived	
boolean
Default: false
Indicates whether the policy is archived.

policyId	
string
Default: "##default"
Represent policy identifier across the system.

policyName	
string
Default: "##default"
Represent policy name.

policyTimeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represent policy time unit.

total	
number <double>
Represents the total amount

used	
number <double>
Represents the balance used amount

userId	
string
Default: "##default"
Represent user identifier across the system.

userName	
string
Default: "##default"
Represent user's username.

workspaceId	
string
Default: "##default"
Represent workspace identifier across the system.

count	
integer <int32>
Represents the count of balances.