# Templates

### Create Label

```yaml
# Create label
create_label:
    name: "{{resp.mime}}"
    type: mime
    color: ignore

create_label:
    name: TODO
    type: endpoint
    icon: todo
    color: blue

# Create label on host
TODO:

# Send Notifications
TODO:

# Playground
create_playground:
    name:      string
    parent_id: string
    type:      string
    expanded:  bool

add_playground:
    parent_id: string
    items: # list of items
        - name:         string
          original_id:  string
          type:         string
          tool_data:    string

```
