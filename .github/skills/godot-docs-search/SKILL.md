---
name: godot-docs-search
description: 'Search the official Godot documentation repository for Godot 4 classes, nodes, methods, properties, signals, tutorials, migration notes, and engine concepts. Use when a user asks where a Godot API or feature is documented, wants the relevant docs page or rst path, asks a Godot question in Chinese, or needs Chinese terms mapped to official English Godot terminology. Prefer repository search in godotengine/godot-docs before broader web lookup.'
argument-hint: 'Describe the Godot class, method, node, property, tutorial topic, or Chinese term to look up'
---

# Godot Docs Search

Search and summarize official Godot documentation from the source repository:
https://github.com/godotengine/godot-docs

This skill is for documentation lookup, terminology mapping, and finding the right official page. Default to Godot 4 terminology and documentation structure unless the user clearly asks about Godot 3. It is not for editing the user's Godot project.

## What This Skill Should Produce

- The best matching official documentation target
- The likely repository path or page type
- The official English Godot term and the matching Chinese phrasing when useful
- A short summary of what the docs say
- Follow-up queries if the first match is incomplete or ambiguous

## When to Use

Use this skill when the user:

- asks where a Godot class, node, method, property, signal, resource, or subsystem is documented
- wants the official docs page for a gameplay or engine concept
- needs the matching file inside `godotengine/godot-docs`
- asks in Chinese and needs the answer mapped to official English Godot terminology
- wants a repository search strategy for Godot docs
- is comparing Godot 3 and Godot 4 naming or migration terms

Default response style:

- explain in Chinese when the prompt is Chinese
- explicitly include official English Godot terms and suggested English queries
- prefer Godot 4 wording unless the user explicitly asks for older docs behavior

Do not use this skill for project code edits, gameplay implementation, or general GDScript architecture work unless the task is specifically about finding official docs.

## Search Targets

Prioritize these areas in `godotengine/godot-docs`:

- `classes/class_*.rst` for API truth
- `tutorials/` for workflows and how-to guidance
- `getting_started/` for onboarding and engine basics
- migration or upgrade pages for Godot 3 to 4 term changes
- glossary-style pages when the user is asking about terminology

## Tool Priority

Use tools in this order:

1. Repository search against `godotengine/godot-docs`
   - Prefer the repo search tool first
   - If available, target `github_repo` with compact English queries
2. Additional repository search refinement
   - Retry with aliases, renamed classes, class-prefixed members, or bilingual terms
3. Web fetch only if repository search does not surface the right document

Do not start with generic web search when repository search is available.

## Core Workflow

### 1. Normalize the User Request

Extract the core lookup target:

- class or node name
- method, property, signal, or enum
- system or concept such as animation, physics, navigation, input, multiplayer
- tutorial topic or workflow
- migration term or renamed API

Also capture:

- whether the user asked in Chinese
- whether the user likely wants API reference or tutorial guidance
- whether the wording looks like Godot 3 terminology
- which bilingual terms should be surfaced in the final answer

### 2. Map Chinese to Official Godot English Terms

Before searching, convert Chinese phrasing into likely official English Godot terminology.

Rules:

- Keep both the original Chinese phrase and the mapped English query
- Search the English term first
- If results are weak, retry with mixed Chinese plus English terms
- Prefer Godot's canonical class and subsystem names over literal translations
- If a phrase is ambiguous, generate 2 to 4 candidate English terms and try the most canonical one first

### 3. Choose the Search Style

Use the request type to decide the first search:

- Class or node lookup:
  Search for `classes/class_<lowercase_name>.rst` first
- Method, property, signal, or enum lookup:
  Search the likely class reference page first, then search repo-wide for the member name
- Concept or workflow lookup:
  Search `tutorials/` or `getting_started/` first, then class reference pages
- Version or migration lookup:
  Search upgrade, migration, and renamed-term pages first

### 4. Refine if the First Search Is Weak

Retry with:

- `snake_case` and spaced variants
- singular and plural forms
- Godot 3 and Godot 4 renamed terms
- class-prefixed combinations such as `CharacterBody2D move_and_slide`
- bilingual queries such as `信号 signal`, `瓦片地图 TileMap`, `导航 NavigationAgent`
- related subsystem terms such as `physics`, `navigation`, `animation`, `input`, `signals`, `resources`

### 5. Synthesize the Answer

Return:

1. Best documentation target
2. Likely repo path or page type
3. Official English term and Chinese wording if the user asked in Chinese
4. Short explanation of why it matches
5. Short summary of what the docs say
6. Alternate queries if certainty is low or the match is indirect

## Chinese to English Mapping

Use these mappings as starting points, then adjust based on context.

### Core Terms

- `节点` -> `Node`
- `场景` -> `Scene`
- `场景树` -> `SceneTree`
- `信号` -> `signal`
- `资源` -> `Resource`
- `脚本` -> `script`
- `导出变量` -> `@export`, `exported property`
- `输入映射` -> `InputMap`
- `输入动作` -> `input action`
- `分组` -> `group`
- `自动加载` -> `autoload`

### 2D

- `2D 节点` -> `Node2D`
- `精灵` -> `Sprite2D`
- `动画精灵` -> `AnimatedSprite2D`
- `瓦片地图` -> `TileMap`, `TileMapLayer`
- `瓦片集` -> `TileSet`
- `相机` -> `Camera2D`
- `视差` -> `Parallax2D`, `ParallaxBackground`
- `标记点` -> `Marker2D`
- `路径` -> `Path2D`
- `路径跟随` -> `PathFollow2D`
- `粒子` -> `GPUParticles2D`, `CPUParticles2D`
- `光照` -> `Light2D`

### 3D

- `3D 节点` -> `Node3D`
- `网格实例` -> `MeshInstance3D`
- `骨骼` -> `Skeleton3D`
- `骨骼绑定点` -> `BoneAttachment3D`
- `相机` -> `Camera3D`
- `方向光` -> `DirectionalLight3D`
- `点光源` -> `OmniLight3D`
- `聚光灯` -> `SpotLight3D`
- `环境` -> `WorldEnvironment`, `Environment`
- `弹簧臂` -> `SpringArm3D`
- `网格库` -> `MeshLibrary`
- `网格地图` -> `GridMap`
- `车辆` -> `VehicleBody3D`
- `软体` -> `SoftBody3D`
- `布娃娃` -> `ragdoll`, `PhysicalBone3D`

### Animation

- `动画播放器` -> `AnimationPlayer`
- `动画树` -> `AnimationTree`
- `状态机` -> `AnimationNodeStateMachine`, `AnimationNodeStateMachinePlayback`
- `状态切换` -> `travel`, `transition`, `state machine transition`
- `混合树` -> `AnimationNodeBlendTree`
- `一维混合空间` -> `AnimationNodeBlendSpace1D`
- `二维混合空间` -> `AnimationNodeBlendSpace2D`
- `过渡节点` -> `AnimationNodeTransition`
- `播放速度` -> `speed_scale`, `time scale`
- `根运动` -> `root motion`

### Navigation

- `导航` -> `navigation`, `NavigationServer`, `NavigationAgent`, `NavigationRegion`
- `导航服务器` -> `NavigationServer2D`, `NavigationServer3D`
- `导航代理` -> `NavigationAgent2D`, `NavigationAgent3D`
- `导航区域` -> `NavigationRegion2D`, `NavigationRegion3D`
- `导航链接` -> `NavigationLink2D`, `NavigationLink3D`
- `导航障碍` -> `NavigationObstacle2D`, `NavigationObstacle3D`
- `导航多边形` -> `NavigationPolygon`
- `导航网格` -> `NavigationMesh`, `navmesh`
- `路径查询` -> `pathfinding`, `path query`
- `避障` -> `avoidance`, `RVO avoidance`
- `烘焙导航网格` -> `bake navigation mesh`, `Bake NavigationPolygon`

### Physics

- `碰撞体` -> `CollisionObject`, `CollisionShape`, or a specific body type depending on context
- `碰撞对象` -> `CollisionObject2D`, `CollisionObject3D`
- `碰撞形状` -> `CollisionShape2D`, `CollisionShape3D`
- `碰撞多边形` -> `CollisionPolygon2D`, `CollisionPolygon3D`
- `形状资源` -> `Shape2D`, `Shape3D`
- `物理体` -> `PhysicsBody2D`, `PhysicsBody3D`
- `角色身体` -> `CharacterBody2D`, `CharacterBody3D`
- `刚体` -> `RigidBody2D`, `RigidBody3D`
- `静态体` -> `StaticBody2D`, `StaticBody3D`
- `可动画静态体` -> `AnimatableBody2D`, `AnimatableBody3D`
- `区域检测` -> `Area2D`, `Area3D`
- `射线检测` -> `RayCast2D`, `RayCast3D`
- `形状投射` -> `ShapeCast2D`, `ShapeCast3D`
- `运动碰撞结果` -> `KinematicCollision2D`, `KinematicCollision3D`
- `物理材质` -> `PhysicsMaterial`
- `碰撞层` -> `collision layer`
- `碰撞掩码` -> `collision mask`
- `接触监视` -> `contact_monitor`
- `重力` -> `gravity`
- `墙面法线` -> `wall normal`
- `滑动移动` -> `move_and_slide`
- `碰撞移动` -> `move_and_collide`

### Multiplayer

- `多人联机` -> `multiplayer`
- `多人同步` -> `multiplayer synchronization`, `MultiplayerSynchronizer`
- `多人生成器` -> `MultiplayerSpawner`
- `多人权限` -> `multiplayer authority`
- `远程过程调用` -> `RPC`, `rpc`
- `多人 API` -> `MultiplayerAPI`
- `场景复制` -> `scene replication`
- `主机` -> `host`, `server`, `authority` depending on context
- `客户端` -> `client`, `peer`
- `对等端` -> `peer`
- `网络唯一 ID` -> `peer ID`, `multiplayer peer ID`

### Common Chinese Query Patterns

- `等待信号` -> `await signal`, `await`, `signal`
- `角色移动` -> `CharacterBody2D movement`, `CharacterBody3D movement`
- `平台跳跃` -> `platformer movement`, `CharacterBody2D`
- `俯视角移动` -> `top-down movement`
- `命中箱` -> `hitbox`
- `受击箱` -> `hurtbox`
- `交互区域` -> `Area2D`, `Area3D`, `body_entered`, `area_entered`

## Version Mapping

If the query uses Godot 3 terms, map them to Godot 4 names when likely relevant. Treat Godot 4 as the default target unless the user explicitly asks for Godot 3 documentation behavior.

Examples:

- `KinematicBody2D` -> `CharacterBody2D`
- `KinematicBody3D` -> `CharacterBody3D`
- `Navigation2DServer` -> `NavigationServer2D`
- `NavigationMeshInstance` -> `NavigationRegion3D`
- `NavigationPolygonInstance` -> `NavigationRegion2D`

For tile workflows in newer docs, try both `TileMap` and `TileMapLayer`.

## Decision Rules

### If the user gives a class or node name

- Search for `classes/class_<lowercase_name>.rst`
- Then search that file for the requested method, property, signal, enum, or tutorial cross-reference
- If no result is found, try likely renamed or replacement classes

### If the user gives only a method, property, or signal

- Search the repo for the exact member name
- Group likely hits by class reference page first
- Prefer results where surrounding context clearly defines the member

### If the user asks a conceptual question

- Search tutorials and guides using the gameplay or engine phrase
- Search again with the likely Godot-specific term
- If the user asked in Chinese, include both the Chinese explanation and the official English term in the answer

### If the result is still weak

- state that the match is approximate
- provide 2 to 4 alternate queries
- separate what is confirmed from what is inferred

## Output Format

Use this structure when returning the result:

1. `Best match:` doc page, class page, or tutorial target
2. `Repo path:` likely `.rst` path or page type
3. `Official term:` official English Godot term plus the matching Chinese term when relevant
4. `Why it matches:` one or two lines
5. `Docs summary:` concise explanation
6. `Try next:` optional fallback queries if needed

## Quality Checks

Before finishing, verify that:

- the answer points to official Godot docs rather than third-party content
- repository search was attempted before broader lookup
- the result matches the user's intent: API reference versus tutorial guidance
- terminology matches the likely Godot version context, with Godot 4 as the default
- Chinese wording has been mapped to official English terminology where possible
- uncertainty is stated clearly if the match is indirect

## Example Prompts

- Search Godot docs for `CharacterBody2D.move_and_slide`
- Find the official docs page for Godot `AnimationTree` state machines
- Search the Godot docs repo for how signals work with `await`
- Find the relevant Godot documentation for multiplayer authority
- Search the docs repo for TileMap navigation setup in Godot 4
- 用中文搜索 Godot 官方文档：瓦片地图导航怎么配置
- 用中文查 Godot 官方文档：信号和 `await` 怎么一起用
- Godot 3 的 `KinematicBody2D` 在 Godot 4 文档里对应什么
