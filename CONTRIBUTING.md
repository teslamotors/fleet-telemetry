# Welcome to Fleet Telemetry contributing guide <!-- omit in toc -->

Thank you for investing your time contributing to Fleet Telemetry

In this guide you will get an overview of the contribution workflow from opening an issue, creating a PR, reviewing, and merging the PR.

## New contributor guide

To get an overview of the project, read the [README](README.md). Here are some resources to help you get started with open source contributions:

- [Finding ways to contribute to open source on GitHub](https://docs.github.com/en/get-started/exploring-projects-on-github/finding-ways-to-contribute-to-open-source-on-github)
- [Set up Git](https://docs.github.com/en/get-started/quickstart/set-up-git)
- [GitHub flow](https://docs.github.com/en/get-started/quickstart/github-flow)
- [Collaborating with pull requests](https://docs.github.com/en/github/collaborating-with-pull-requests)


## Getting started

### How to contribute ?
We will accept contributions from anybody assuming they respect our policies and help improve the platform. here's the different types of contribution we support.

#### :lady_beetle: Issues
[Issues](https://docs.github.com/en/github/managing-your-work-on-github/about-issues) are used to track tasks that contributors can help with. If an issue has a triage label, we haven't reviewed it yet, and you shouldn't begin work on it.

If you've found something in the code that should be updated, search open issues to see if someone else has reported the same thing. If it's something new, open an issue. We'll use the issue to have a conversation about the problem you are having or want to fix.

#### :hammer_and_wrench: Pull requests
A [pull request](https://docs.github.com/en/github/collaborating-with-issues-and-pull-requests/about-pull-requests) is a way to suggest changes in our repository. When we merge those changes, it will kick of a new build. Currently we will manually push the update to docker to ensure a good understanding of release should be a major, minor or patch. We might automate this process in the future.


#### :question: Support
We are a small team working hard to keep up with the documentation demands of a continuously changing product. Unfortunately, we can't help with support questions in this repository. If you are experiencing a problem unrelated to the code hosted here please [contact Tesla Support directly](https://https://www.tesla.com/support). Any issues, discussions, or pull requests opened here requesting support will be given information about how to contact Tesla Support, then closed and locked.

#### :earth_asia: Translations

This website is internationalized and available in multiple languages. The source content in this repository is written in English.

**We do not currently accept contributions for translated content**.

### Issues

#### Create a new issue

If you spot a problem within this repository (code, documentation, style, etc.), [search if an issue already exists](https://docs.github.com/en/github/searching-for-information-on-github/searching-on-github/searching-issues-and-pull-requests#search-by-the-title-body-or-comments).

Issues created here should only target the Fleet Telemetry project, if you have any issues with your vehicle please [contact Tesla Support directly](https://https://www.tesla.com/support).

DO NOT: When creating or responding to a Github Issue DO NOT post any personal information, account information, vehicle information or anything that could identify you, your collaborators or your customers.

#### Solve an issue

Scan through our [existing issues](https://github.com/teslamotors/fleet-telemetry/issues) to find one that interests you. As a general rule, we don’t assign issues to anyone. If you find an issue to work on, you are welcome to open a PR with a fix.

### Make Changes

#### Make changes in the UI

Click **Make a contribution** at the bottom of any docs page to make small changes such as a typo, sentence fix, or a broken link. This takes you to the `.md` file where you can make your changes and [create a pull request](#pull-request) for a review.

#### Make changes in a codespace

For more information about using a codespace for working on GitHub documentation, see "[Working in a codespace](https://github.com/github/docs/blob/main/contributing/codespace.md)."

#### Make changes locally

1. Fork the repository.
- Using GitHub Desktop:
  - [Getting started with GitHub Desktop](https://docs.github.com/en/desktop/installing-and-configuring-github-desktop/getting-started-with-github-desktop) will guide you through setting up Desktop.
  - Once Desktop is set up, you can use it to [fork the repo](https://docs.github.com/en/desktop/contributing-and-collaborating-using-github-desktop/cloning-and-forking-repositories-from-github-desktop)!

- Using the command line:
  - [Fork the repo](https://docs.github.com/en/github/getting-started-with-github/fork-a-repo#fork-an-example-repository) so that you can make your changes without affecting the original project until you're ready to merge them.

2. Create a working branch and start with your changes!

### Commit your update

Commit the changes once you are happy with them.

### Pull Request

When you're finished with the changes, create a pull request, also known as a PR.
- Fill the "Ready for review" template so that we can review your PR. This template helps reviewers understand your changes as well as the purpose of your pull request.
- Don't forget to [link PR to issue](https://docs.github.com/en/issues/tracking-your-work-with-issues/linking-a-pull-request-to-an-issue) if you are solving one.
- Enable the checkbox to [allow maintainer edits](https://docs.github.com/en/github/collaborating-with-issues-and-pull-requests/allowing-changes-to-a-pull-request-branch-created-from-a-fork) so the branch can be updated for a merge.
Once you submit your PR, a Docs team member will review your proposal. We may ask questions or request additional information.
- We may ask for changes to be made before a PR can be merged, either using [suggested changes](https://docs.github.com/en/github/collaborating-with-issues-and-pull-requests/incorporating-feedback-in-your-pull-request) or pull request comments. You can apply suggested changes directly through the UI. You can make any other changes in your fork, then commit them to your branch.
- As you update your PR and apply changes, mark each conversation as [resolved](https://docs.github.com/en/github/collaborating-with-issues-and-pull-requests/commenting-on-a-pull-request#resolving-conversations).
- If you run into any merge issues, checkout this [git tutorial](https://github.com/skills/resolve-merge-conflicts) to help you resolve merge conflicts and other issues.

### Your PR is merged!

Once your PR is merged, your contributions will be publicly visible on the [Fleet Telemetry Github](https://github.com/teslamotors/fleet-telemetry). It will be published in a future [release](https://github.com/teslamotors/fleet-telemetry/releases).
