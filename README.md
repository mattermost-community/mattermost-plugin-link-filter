# Mattermost Plugin Link Filter

This plugin allows you to filter links on your Mattermost server. The plugin checks all links in messages for matches against the configured `Allowed Protocols list`.

## Installation

1. Go to the [releases page of this Github repository](https://github.com/Brightscout/mattermost-plugin-link-filter/releases) and download the latest release for your Mattermost server.
2. Upload this file in the Mattermost System Console under **System Console > Plugins > Management** to install the plugin. To learn more about how to upload a plugin, [see the documentation](https://docs.mattermost.com/administration/plugins.html#plugin-uploads).
3. Activate the plugin at **System Console > Plugins > Management**.


### Usage

You can edit the allowed portocol list in **System Console > Plugins > Profanity Filter > Allowed Protocols list**.

For example, `http,https` will allow messages with links like `https://github.com` or `http://github.com` but reject posts containing links like `s3://YourS3Bucket/dir/filename.filetype`.
