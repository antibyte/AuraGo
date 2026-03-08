Always cleanup temporary files, logs.
Never commit or store sensitive data, credentials, or personally identifiable information in the repository. Check before commiting.
Use make_deploy script to build binaries and upload to test server
For the Web-Ui there is a help text file. Keep it up to date.
Do not forget to update tool manuals and prompts if you add new integrations oder tools for the agent.
Security by design: Always consider security implications when adding new tools, integrations, or code. Avoid introducing vulnerabilities or exposing sensitive data.
New tools and integrations that can potenially change or delete data or perform critical operations should habe a read only toggle.
If the tool or integration needs more granular permissions to be usefull, use separate toggles for read ,write, change and delete access. 
all tools and integrations should have a toggle to activate them unless they are essential for the system to fuction properly.
Do all Web UI work as user friendly as possible. Avoid too technical jargon and provide clear instructions and feedback to the user. Do not break the style of the UI. Your changes should be a masterpiece of UX design and fit seamlessly into the existing interface. If you see bad UX in the existing UI feel free to improve it as long as you keep the overall style and design consistent.
If anything passes external content to the agent implement the necessary safety measures to prevent prompt injection and other attacks. Always assume that external content is potentially malicious and treat it accordingly. Use the `<external_data>` wrapper for all untrusted content and never allow it to influence agent behavior or tool calls directly.
always store credentials and sensitive data directly in the secrets vault, never in the code or configuration files.
