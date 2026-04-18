# LikeC4 DSL Syntax Reference

Website: https://likec4.dev/  
File extensions: `.likec4` or `.c4`

## Specification Block

Defines custom element kinds, relationship kinds, and tags. Must appear before model.

```likec4
specification {
  element actor
  element system
  element service
  element component
  element database {
    style { shape storage }
  }
  relationship async
  tag deprecated
}
```

Built-in shapes: `rectangle` (default), `storage`, `browser`, `mobile`, `cylinder`, `queue`, `person`.

## Model Block

Declares elements and their relationships.

### Elements

```likec4
model {
  // Top-level elements
  customer = actor 'Customer' {
    description 'End user of the platform'
  }

  // Nested elements (container pattern)
  saas = system 'Our SaaS' {
    description 'Main platform'

    ui = component 'Frontend' {
      technology 'React'
      style { shape browser }
    }
    api = service 'API Gateway' {
      technology 'gRPC'
    }
    db = database 'PostgreSQL'
  }
}
```

### Relationships

```likec4
// Basic
a -> b 'label'

// With description and technology
a -> b 'label' 'description' 'technology'

// Typed relationship (using relationship kind from specification)
a -[async]-> b 'publishes event'
// or dot syntax:
a .async b 'publishes event'

// Sourceless inside nesting (implicit this ->)
saas = system 'SaaS' {
  api = service 'API'
  db = database 'DB'
  api -> db 'reads/writes'
}

// Top-level cross-references
customer -> saas.ui 'opens in browser'
```

## Views Block

### Static Views

```likec4
views {
  // Landscape view — shows everything
  view index {
    title 'System Landscape'
    include *
  }

  // Scoped view — shows internals of a specific element
  view of saas {
    title 'SaaS Internals'
    include *
  }

  // Extended view — inherits from another view and adds elements
  view detailedApi extends index {
    include saas.api
  }

  // Include/exclude control
  view filtered {
    include *
    exclude saas.db
    style customer { color muted }
  }
}
```

### Dynamic Views (Flow/Sequence Diagrams)

Steps are numbered automatically. Each `->` is a step in the sequence.

```likec4
dynamic view myFlow {
  title 'My Protocol Flow'

  // Sequential steps
  client -> server 'Request'
  server -> database 'Query'
  database -> server 'Result'
  server -> client 'Response'
}
```

**With notes (markdown supported):**

```likec4
dynamic view withNotes {
  client -> server 'RegisterRequest' {
    notes 'First message on the stream. **Must** be RegisterRequest for new agents.'
  }
  server -> client 'RegistrationCode' {
    notes 'Agent displays the 6-char code to the operator.'
  }
}
```

**Parallel steps:**

```likec4
dynamic view withParallel {
  ui -> api 'request dashboard'
  parallel {
    api -> cache 'check cache'
    api -> db 'query data'
  }
  api -> ui 'aggregated response'
}
```

**Navigation between views (drill-down):**

```likec4
dynamic view overview {
  ui -> api 'process request' {
    navigateTo detailedFlow
  }
}

dynamic view detailedFlow {
  title 'Detailed API Handling'
  api -> auth 'validate'
  api -> db 'query'
}
```

## Styling

### Inline element styles

```likec4
myElement = component 'Name' {
  style {
    shape browser
    color green
    opacity 50%
  }
}
```

### View-level style overrides

```likec4
view myView {
  include *
  style customer {
    color muted
  }
  style saas.db {
    color red
  }
}
```

Available colors: `primary`, `secondary`, `muted`, `red`, `green`, `blue`, `amber`, `gray`, `slate`, `indigo`.

## Multi-File Projects

LikeC4 auto-discovers and merges all `*.likec4` files in a directory tree. There is no import/include syntax — everything is merged automatically.

Each file can contain any combination of `specification`, `model`, and `views` blocks. Use `extend` to add children to elements defined in other files.

**Typical project structure:**

```
docs/
  likec4/
    spec.likec4              # specification { ... }
    model.likec4             # model { ... } — top-level elements
    model.backend.likec4     # model { extend backend { ... } }
    model.frontend.likec4    # model { extend frontend { ... } }
    views.likec4             # views { ... } — static views
    views.flows.likec4       # views { ... } — dynamic flow views
```

**Extending elements across files:**

```likec4
// model.likec4
model {
  backend = system 'Backend'
}

// model.backend.likec4
model {
  extend backend {
    api = service 'API'
    db = database 'Database'
    api -> db 'queries'
  }
}
```

**Directory scanning:**

- `npx likec4 serve` scans the current working directory recursively. Pass a path to narrow it: `npx likec4 serve ./docs/likec4`
- VS Code extension scans the workspace root for `*.likec4` files.
- CLI accepts `-w` / `--workspace` flag to specify the directory.

**Configuration file (optional):**

Supported names: `.likec4rc`, `.likec4.config.json`, `likec4.config.json`, `likec4.config.js`, `likec4.config.mjs`, `likec4.config.ts`, `likec4.config.mts`.

Schema: `https://likec4.dev/schemas/config.json`

```json
{
  "$schema": "https://likec4.dev/schemas/config.json",
  "name": "my-project",
  "title": "My Project",
  "exclude": ["**/node_modules/**"],
  "implicitViews": false
}
```

| Option | Type | Default | Purpose |
|---|---|---|---|
| `name` | string | (required) | Unique project identifier |
| `title` | string | -- | Human-readable name |
| `exclude` | string[] | -- | Glob patterns to exclude from scanning |
| `include` | object | -- | Include dirs outside project (`paths`, `maxDepth`, `fileThreshold`) |
| `implicitViews` | boolean | `false` | Auto-generate scoped views for all elements |
| `inferTechnologyFromIcon` | boolean | `true` | Auto-derive technology labels from icons |
| `imageAliases` | object | -- | Shortcuts for image paths (e.g., `"@": "./images"`) |
| `styles` | object | -- | Theme colors, sizes, element/relationship defaults |
| `extends` | string/array | -- | Share styles across projects (JSON only) |
| `landingPage` | object | -- | Configure root URL redirect |
| `generators` | object | -- | Custom code generation (experimental, `.ts` config only) |

## Practical Tips

- Each `dynamic view` is a self-contained flow diagram — great for documenting RPC sequences.
- Use `notes` liberally on dynamic view steps for protocol details.
- Use `navigateTo` to link overview diagrams to detailed sub-flows.
- Preview with the LikeC4 VS Code extension or `npx likec4 serve`.
- Export with `npx likec4 export png` or `npx likec4 build` for a static site.
