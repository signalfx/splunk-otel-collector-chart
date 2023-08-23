
# Developer Guide for Helm Chart Examples

This document provides an overview for developers on how the Helm Chart examples are managed,
rendered, and the conventions used.

## Rendering Examples with the Supplied Script

The script `render-examples.sh` has been provided to facilitate the rendering of Helm templates for
each example. A developer can run `make render` to execute `render-examples.sh`.

Here's a brief overview of how the script functions:

1. **Setup**: The script first determines its own directory to find the examples.
2. **Rendering**: For each example directory:
  - Previously rendered manifests are cleared.
  - The Helm template command is executed to generate rendered files.
  - Rendered files are moved to the appropriate directories or deleted if the filename contains `-values.norender.yaml`.
  - If subcharts exist, their rendered files are also processed.
  - Temporary spaces used during the process are cleaned up.
  - Any failure at any step results in an error message, and the script exits with an error code.
3. **Parallel Execution**: The script renders examples in parallel to speed up the process. Once
all tasks are initiated, the script waits for all of them to complete.
4. **Post-rendering Check**: After rendering, the script checks to ensure that each example has a `rendered_manifests`
directory. If any are missing, it signifies a failure.

## File Naming Conventions

- Files ending with `-values.yaml` will be rendered by the script.
- Files ending with `-values.norender.yaml` will not be rendered by the script. This is useful for
examples that are derivatives or for any other reason you might not want them rendered.

### Derivative Examples and Code Review Efficiency

When adding derivative examples that showcase specific cases, it's recommended to use the `-values.norender.yaml`
suffix for the filename. By doing this, the example won't be rendered by the script, ensuring that
code review diffs remain compact and focused. This practice prevents the inclusion of derivative
rendered manifests, which can bloat the diffs and make reviews more challenging.

### Adding Examples Without Rendering

If you want to add an example but don't wish it to be rendered by the script, ensure the file
follows the `-values.norender.yaml` naming convention. This will prevent the `render-examples.sh`
script from processing it.
