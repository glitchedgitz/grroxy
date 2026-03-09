<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { EditorView, placeholder as cmPlaceholder } from "@codemirror/view";
  import { EditorState } from "@codemirror/state";
  import { json } from "@codemirror/lang-json";
  import { basicSetup } from "codemirror";
  import { httpEditorTheme } from "./cm-http-theme";

  let {
    value = $bindable(""),
    readonly = false,
    placeholder = "",
  }: {
    value: string;
    readonly?: boolean;
    placeholder?: string;
  } = $props();

  let container: HTMLDivElement;
  let view: EditorView;
  let updating = false;

  onMount(() => {
    const extensions = [
      basicSetup,
      json(),
      httpEditorTheme,
      EditorView.lineWrapping,
    ];

    if (placeholder) {
      extensions.push(cmPlaceholder(placeholder));
    }

    if (readonly) {
      extensions.push(EditorState.readOnly.of(true));
      extensions.push(EditorView.editable.of(false));
    } else {
      extensions.push(
        EditorView.updateListener.of((update) => {
          if (update.docChanged && !updating) {
            updating = true;
            value = update.state.doc.toString();
            updating = false;
          }
        }),
      );
    }

    view = new EditorView({
      state: EditorState.create({
        doc: value,
        extensions,
      }),
      parent: container,
    });
  });

  onDestroy(() => {
    view?.destroy();
  });

  $effect(() => {
    if (view && !updating) {
      const current = view.state.doc.toString();
      if (value !== current) {
        updating = true;
        view.dispatch({
          changes: {
            from: 0,
            to: current.length,
            insert: value,
          },
        });
        updating = false;
      }
    }
  });
</script>

<div bind:this={container} class="h-full w-full overflow-auto"></div>
