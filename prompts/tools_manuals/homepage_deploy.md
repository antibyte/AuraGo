# homepage_deploy

Deploy homepage projects through configured deploy targets.

Supported operations are `deploy`, `deploy_netlify`, and `deploy_vercel`. Build and package from the homepage workspace; remote deployment still follows the deployment toggles in configuration.

Project-scoped build, dev, publish, and deploy flows require a workspace-relative `project_dir`, for example `my-site`. Use the actual project directory from the homepage workspace or registry; do not pass absolute paths, host paths, `/workspace/...`, or guessed folder names.

**After a successful deployment, call `homepage_registry` → `add_history` with `entry_type: milestone` and record the deployed URL.**
