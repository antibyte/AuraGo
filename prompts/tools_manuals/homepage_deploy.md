# homepage_deploy

Deploy homepage projects through configured deploy targets.

Supported operations are `deploy`, `deploy_netlify`, and `deploy_vercel`. Build and package from the homepage workspace; remote deployment still follows the deployment toggles in configuration.

**After a successful deployment, call `homepage_registry` → `add_history` with `entry_type: milestone` and record the deployed URL.**
