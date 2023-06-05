# IAM - 账户和权限

先了解和OpenPitrix业务相关的几个核心概念。然后以app商店的app增删改查为例，展示了超级用户/应用管理员/普通用户如何通过iam服务进行鉴权。

-----
## 服务的URL规范

OpenPitrix后端服务采用GRPC定义接口，然后通过grpc-gateway映射到URL，对外对应Rest风格的服务。

URL的映射规则如下：

```
/api/ServiceName.ServiceMethodName/group1.group2.group3/user_id
```

其中`/api`空间下表示GRPC提供的Rest服务，ServiceName表示服务的名字（序言全局唯一），
ServiceMethodName表示服务中方法的名字，而`/group1.group2.group3/user_id`表示Rest资源对的名字空间。

在OpenPitrix中，Rest资源对应的路径`/group1.group2.group3/user_id`和账户的组织部分是对应的关系。
比如`/group1.group2.group3`部门管理的资源路径是`/QingCloud应用中心/内部组织/应用平台开发部`。

下面以AppManager服务的CreateApp方法为例，展示如何对应到Rest路径：

```protobuf
service AppManager {
	rpc CreateApp (CreateAppRequest) returns (CreateAppResponse) {
		option (google.api.http) = {
			post: "/api/AppManager.CreateApp/{app_path=**}"
			body: "*"
		};
	}
}

message CreateAppRequest {
	string app_path = 1;
}
```

CreateApp方法被映射为`/api/AppManager.CreateApp/{app_path=**}`路径，其中`/api/AppManager.CreateApp/`之后的路径被填充到`CreateAppRequest.app_path`参数中。

因此下面的请求：

```
POST
`/api/AppManager.CreateApp/group1.group2.group3/chai
```

对应下面的GRPC调用：

```go
client.CreateApp(&pb.CreateAppRequest{
	AppPath: "/group1.group2.group3/chai",
})
```

其中`/group1.group2.group3/chai`对应的资源名字空间可以省略。如果省略了名字空间，那么将根据user对应的默认名字空间填充（这个信息在更上层的`openpitrix/iam`模块保存）。

AppManager服务的实现者，不需要关心是谁调用了这个方法，也不需要关心资源对应的组织部分是否存在。
因为真正的Rest调用是从Gateway，经过登陆验证、IAM鉴权之后才达到AppManager服务的，因此只要能够调用方法就说明是有权限调用的。

在IAM通过鉴权之后，Gateway需要将用户的user_id信息放到Http请求的头部，AppManager.CreateApp在被执行的时候，可以通过Context参数取到user_id信息。AppManager的实现者可以根据user_id信息进行二次鉴权（业务服务自行决定如何使用user_id）。

AppManager的实现者，需要根据AppPath的信息组织数据，因为后续的删除和修改也是根据同样的AppPath进行操作。

-----
## 组织部门和用户

![](./images/iam-group-user.png)

对应 im 账号管理模块中的group和user数据库表的概念，其中group是树形的组，user是在树形组织结构的叶子位置。

group表中有个gid唯一表示了组的位置信息，比如下面每一行为一个组的gid，每个gid对应组织结构中组的路径：

```
{gid:group1, parent_id:group1, name:QingCloud应用中心}
{gid:group2, parent_id:group1, name:内部组织}
{gid:group3, parent_id:group2, name:应用平台开发部}
{gid:group4, parent_id:group3, name:OpenPitrix}
{gid:group5, parent_id:group3, name:AppCenter}
{gid:group6, parent_id:group3, name:KubeSphere}

{gid:group7, parent_id:group7, name:外部组织}
{gid:group8, parent_id:group7, name:应用服务商}
{gid:group9, parent_id:group7, name:普通用户}
```

上面的数据对应下面的树形组织结构（其中根group0的父亲是自己）。

```
QingCloud应用中心
QingCloud应用中心.内部组织
QingCloud应用中心.内部组织.应用平台开发部
QingCloud应用中心.内部组织.应用平台开发部.OpenPitrix
QingCloud应用中心.内部组织.应用平台开发部.AppCenter
QingCloud应用中心.内部组织.应用平台开发部.KubeSphere
QingCloud应用中心.内部组织.云平台Iaas开发部
QingCloud应用中心.内部组织.云平台Iaas开发部.???

外部组织
外部组织.应用服务商
外部组织.普通用户
```

group表中有个gid_parent唯一表示了父亲组的位置信息。如果gid_parent和gid相同，则表示为根组。比如下面几个组的对应关系：

```
# QingCloud应用中心
gid: group1
gid_parent: group1
path: QingCloud应用中心

# OpenPitrix
gid: group1
gid_parent: group4
path: QingCloud应用中心.内部组织.应用平台开发部.OpenPitrix
```

用户处于组织结构的叶子节点。为了便于管理，OpenPitrix预置了“超级管理员”/“应用服务商”/“普通用户”。为了便于理解，我们假设reno用户拥有“超级管理员”权限，ray用户拥有“应用服务商”权限，而chai用户拥有“普通用户”。

在user表中，uid表示用户唯一的标识，gid表示用户属于的组。那么以上三个用户的信息如下：

```
# admin
uid: reno
group_path: QingCloud应用中心.内部组织.X
name: reno

# isv
uid: ray
group_path: QingCloud应用中心.内部组织.应用平台开发部
name: ray

# user
uid: chai
group_path: QingCloud应用中心.内部组织.应用平台开发部.OpenPitrix
name: chaishushan
```

目前，三个用户没有任何的操作权限。如果需要给他们配置不同级别的权限，需要给他们赋予具有不同权限的角色。

-----
## 角色管理

![](./images/iam-role.png)

图中有“超级管理员”/“应用服务商”/“普通用户”三种角色。每个角色有一个唯一的角色名字，这三个角色的名字分别是role_root/role_isv/role_user。对应 am 权限管理模块中的role数据库表的概念。

我们可以将reno/ray/chai分布绑定到不同的角色：

```
reno <--> role_root
ray  <--> role_isv
chai <--> role_user
```

然后给每个角色附带一组`操作权限`规则：

```
role_root - 超级管理员
	action_rule:
		method_pattern: *.*
		namespace_pattern: [
			"/**"
		]
role_isv  - 应用服务商
	action_rule:
		method_pattern: AppManager.CreateApp
		namespace_pattern: [
			"$group_path/**"
		]
role_user - 普通用户
	action_rule:
		method_pattern: *.*
		namespace_pattern: [
			"$group_path/$uid/**"
		]
```

在操作规则中`$group_path`表示账户所在的组织部门的绝对路径，`$uid`表示账号的ID。

-----
## 操作权限

![](./images/iam-role-action-rule.png)

每个功能对应对应 am 权限管理模块中的 action_rule 数据库表的概念。

比如应用商店管理员角色中的`增加应用`功能对应 action_rule 表中名为 `action_rule_isv_app_adder` 这个规则，规则的内容如下：

```
# action_rule_isv_app_adder
action_rule:
	name: action_rule_isv_app_adder
	method_pattern: AppManager.CreateApp
	namespace_pattern: [
		"$gid/**"
	]
```

然后将 `action_rule_isv_app_adder` 操作权限绑定到 `role_isv` 角色。

-----

## 鉴权流程

那么对于 ray 用户，他已经被绑定到了 `role_isv` 角色，因此将拥有`action_rule_isv_app_adder` 操作权限。

而 chai 用户所在到部门 `QingCloud应用中心.内部组织.应用平台开发部.OpenPitrix` 是 ray 所在部门 `QingCloud应用中心.内部组织.应用平台开发部` 的子部门。因此 ray 可以新建一个 app 应用，并将 app 放到 chai 用户对应到资源空间下面。

首先为AppManager服务的每个方法映射一个唯一到URL：

```protobuf
service AppManager {
	rpc CreateApp (CreateAppRequest) returns (CreateAppResponse) {
		option (google.api.http) = {
			post: "/api/AppManager.CreateApp/{app_org_path=**}"
			body: "*"
		};
	}
}

message CreateAppRequest {
	string app_owner_id = 1;  // 用户ID
	string app_org_path = 2; // App所在的路径

	// 其它参数不参与鉴权
}
```

在CreateAppRequest中，org_path对应app所属的组织结构的路径。如果org_path为空的话，则根据user_id查询所属的组织结构的路径。

下面是可能的请求URL的模式：

```
/api/AppManager.CreateApp?app_owner_id=chai
/api/AppManager.CreateApp/group1.group2.group3.group4/chai
```

因此 ray 通过以下的POST请求完成上面的操作（为chai用户创建一个app）：

```
uid: ray
POST /api/AppManager.CreateApp/group1.group2.group3.group4/chai
```

在OpenPitrix系统到gateway服务收到 ray 上述到请求之后：

1. 首先对 ray 进行登陆验证（此处细节省略），如果登陆成功则到下一步
1. 将用户对账号和请求的URL，交给 `openpitrix/pkg/iam.CanDoAction` 包做鉴权（此包待实现）
1. 上一步对包会将URL解码为两个部分：`AppManager.CreateApp`表示服务的方法；`/QingCloud应用中心/内部组织/应用平台开发部/OpenPitrix`表示资源对应的名字空间
1. 将 uid(ray)/服务的方法/资源对应的名字空间 发送给 am 服务做鉴权
1. am 服务查询内部的角色用户绑定关系表，得知 ray 被当定到了 role_isv 角色
1. 而 role_isv 角色包含了 action_rule_isv_app_adder 操作权限

action_rule_isv_app_adder 操作权限在前面已经定义过，内容如下：

```
# action_rule_isv_app_adder
action_rule:
	name: action_rule_isv_app_adder
	method_pattern: AppManager.CreateApp
	namespace_pattern: [
		"$group_path/**"
	]
```

那么说明 ray 用户绑定的 role_isv 角色拥有 AppManager.CreateApp 服务方法的调用权限，
同时对名字空间 `$group_path/**` 下的资源有操作权限。

将名字空间 `$group_path/**` 中的gid展开为 ray 所在的部门组织，最终操作权限对应的名字空间规则为
`/group1.group2.group3.group4/**`，其中`**`表示可以跨越路径分割符进行任意匹配。

因此ray具备下面操作的权限：

```
uid: ray
POST /api/AppManager.CreateApp/group1.group2.group3.group4
POST /api/AppManager.CreateApp?app_owner_id=chai
```

因此 iam 模块将放行，交给 AppManager 的服务进行业务处理。


-----
## 用例1: 创建App

假设需要创建以下App资源

```
/api/QingCloud应用中心/group1.group2.group3.group4/chai/simple-app
```

系统有3个用户：

1. reno: 超级管理员，所在组为：`/QingCloud应用中心/内部组织`
2. ray: ISV管理员，所在组为：`/QingCloud应用中心/内部组织/应用平台开发部`
3. chai: 普通用户，所在组为 `/QingCloud应用中心/内部组织/应用平台开发部/OpenPitrix`

创建App对应的服务为：

```protobuf
service AppManager {
	rpc CreateApp (CreateAppRequest) returns (CreateAppResponse) {
		option (google.api.http) = {
			post: "/api/AppManager.CreateApp/{app_path=**}"
			body: "*"
		};
	}
}

message CreateAppRequest {
	string app_path = 1; // App所在的路径

	// 其它参数不参与鉴权
}
```

### A，由超级管理员reno创建

```
POST /api/AppManager.CreateApp/group1.group2.group3.group4/chai/simple-app
```

reno被绑定到了role_root角色，角色的操作权限如下：

```
role_root - 超级管理员
	action_rule:
		method_pattern: "/api/*.*"
		namespace_pattern: [
			"/**"
		]
```

鉴权流程：

1. 验证登陆
2. 根据uid找到role_root角色
3. 根据role_root角色获取操作的模式为`/api/*.*`，可以匹配URL中的"/api/AppManager.CreateApp"部分（POST不需要处理）
4. 数据模式为`/**`，可以匹配 `/group1.group2.group3.group4/chai/simple-app` 部分
5. 正常调用AppManager服务（不需要了解IAM的信息）

### B，由商店管理员ray创建

ray被绑定到了role_isv角色，角色的操作权限如下：

```
role_isv  - 应用服务商
	action_rule:
		method_pattern: "/api/*.*"
		namespace_pattern: [
			"$gid/**"
		]
```

鉴权流程：

1. 验证登陆
2. 根据uid找到role_isv角色
3. 根据role_isv角色获取操作的模式为`/api/*.*`，可以匹配URL中的"/api/AppManager.CreateApp"部分（POST不需要处理）
4. 数据模式为`$gid/**`，将`$gid`展开为ray所在的组，对应`/group1.group2.group3.group4/**`，因此可以匹配`/group1.group2.group3.group4/chai/simple-app` 部分
5. 正常调用AppManager服务（不需要了解IAM的信息）

### C，由普通用户chai创建

chai被绑定到了role_user角色，角色的操作权限如下：

```
role_user  - 普通成员
	action_rule:
		method_pattern: "/api/*.*"
		namespace_pattern: [
			"$gid/$uid/**"
		]
```

鉴权流程：

1. 验证登陆
2. 根据uid找到role_user角色
3. 根据role_user角色获取操作的模式为`/api/*.*`，可以匹配URL中的"/api/AppManager.CreateApp"部分（POST不需要处理）
4. 数据模式为`$gid/$uid/**`，将`$gid`和`$uid`用chai所在的组和ID替代，对应`/group1.group2.group3.group4/chai/**`，因此可以匹配`/group1.group2.group3.group4/chai/simple-app` 部分
5. 正常调用AppManager服务（不需要了解IAM的信息）

-----
## 用例2: 删除App

假设需要删除以下App资源

```
/group1.group2.group3.group4/chai/simple-app
```

系统有3个用户：

1. reno: 超级管理员，所在组为：`/QingCloud应用中心/内部组织`
2. ray: ISV管理员，所在组为：`/QingCloud应用中心/内部组织/应用平台开发部`
3. chai: 普通用户，所在组为 `/QingCloud应用中心/内部组织/应用平台开发部/OpenPitrix`

删除App对应的服务为：

```protobuf
service AppManager {
	rpc DeleteApp (DeleteAppRequest) returns (DeleteAppResponse) {
		option (google.api.http) = {
			delete: "/api/AppManager.DeleteApp/{app_path=**}"
			body: "*"
		};
	}
}
message DeleteAppRequest {
	string app_path = 1; // App所在的路径

	// 其它参数不参与鉴权
}
```


### A，由超级管理员reno删除

```
DELETE /api/AppManager.DeleteAppRequest/group1.group2.group3.group4/chai/simple-app
```

reno被绑定到了role_root角色，角色的操作权限如下：

```
role_root - 超级管理员
	action_rule:
		method_pattern: "/api/*.*"
		namespace_pattern: [
			"/**"
		]
```

鉴权流程：

1. 验证登陆
2. 根据uid找到role_root角色
3. 根据role_root角色获取操作的模式为`/api/*.*`，可以匹配URL中的"/api/AppManager.DeleteAppRequest"部分（DELETE不需要处理）
4. 数据模式为`/**`，可以匹配 `/group1.group2.group3.group4/chai/simple-app` 部分
5. 正常调用AppManager服务（不需要了解IAM的信息）

### B，由商店管理员ray删除

ray被绑定到了role_isv角色，角色的操作权限如下：

```
role_isv  - 应用服务商
	action_rule:
		method_pattern: "/api/*.*"
		namespace_pattern: [
			"$gid/**"
		]
```

鉴权流程：

1. 验证登陆
2. 根据uid找到role_isv角色
3. 根据role_isv角色获取操作的模式为`/api/*.*`，可以匹配URL中的"/api/AppManager.DeleteApp"部分（DELETE不需要处理）
4. 数据模式为`$gid/**`，将`$gid`展开为ray所在的组，对应`/group1.group2.group3.group4/**`，因此可以匹配`/group1.group2.group3.group4/chai/simple-app` 部分
5. 正常调用AppManager服务（不需要了解IAM的信息）

### C，由普通用户chai删除

chai被绑定到了role_user角色，角色的操作权限如下：

```
role_user  - 普通成员
	action_rule:
		method_pattern: "/api/*.*"
		namespace_pattern: [
			"$gid/$uid/**"
		]
```

鉴权流程：

1. 验证登陆
2. 根据uid找到role_user角色
3. 根据role_user角色获取操作的模式为`/api/*.*`，可以匹配URL中的"/api/AppManager.CreateApp"部分（DELETE不需要处理）
4. 数据模式为`$gid/$uid/**`，将`$gid`和`$uid`用chai所在的组和ID替代，对应`/group1.group2.group3.group4/chai/**`，因此可以匹配`/group1.group2.group3.group4/chai/simple-app` 部分
5. 正常调用AppManager服务（不需要了解IAM的信息）

-----
## 用例3: 查App列表

服务为：

```protobuf
service AppManager {
	rpc DescribeApps (DescribeAppsRequest) returns (DescribeAppsResponse) {
		option (google.api.http) = {
			get: "/api/AppManager.DescribeApps/{app_path=**}"
			body: "*"
		};
	}
}
message DescribeAppsRequest {
	string app_path = 1; // App所在的路径

	// 其它参数不参与鉴权
}
```

reno查全部App列表：

```
GET /api/AppManager.DescribeApps/group1
```

ray查所在组的App列表：

```
GET /api/AppManager.DescribeApps/group1.group2.group3
```

ray查chai的App列表：

```
GET /api/AppManager.DescribeApps/group1.group2.group3.group4/chai
```

每个用户如果省略了资源的名字空间，那么用默认最大的权限空间路径来填充。

-----
## 用例4: 修改App信息

服务为：

```protobuf
service AppManager {
	rpc ModifyApp (ModifyAppRequest) returns (ModifyAppResponse) {
		option (google.api.http) = {
			patch: "/api/AppManager.ModifyAppApp/{app_path=**}"
			body: "*"
		};
	}
}
message ModifyAppAppRequest {
	string app_path = 1; // App所在的路径

	string description = 2; // 描述信息，可以修改

	// 其它参数不参与鉴权
}
```

reno修改chai的App信息：

```
PATCH /api/AppManager.ModifyApp/group1.group2.group3.group4/chai/simple-app?description=abc
```

ray修改chai的App信息：

```
PATCH /api/AppManager.ModifyApp/group1.group2.group3.group4/chai/simple-app?description=abc
```

ray修改chai的App信息：

```
PATCH /api/AppManager.ModifyApp/group1.group2.group3.group4/chai/simple-app?description=abc
```

----
# AppManager 服务的注意事项

每个App需要保存资源所在的路径，该路径和组织部分的结构要保持一致。

比如chai要创建simple-app应用，根据chai所在的组可以得知应用对应的资源路径为“/group1.group2.group3.group4/chai/simple-app”。

再组合服务的方法对应的Rest-API得到路径：“/api/AppManager.CreateApp/group1.group2.group3.group4/chai/simple-app”

从数据库表查App列表时，根据App对应的资源路径做前缀匹配即可。

AppManager只要遵循Rest-API的映射规则，并不需要了解IAM服务。
