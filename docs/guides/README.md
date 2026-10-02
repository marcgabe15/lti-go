# Guides

Task-oriented how-tos for using `lti-go`. If you're looking for *why*
the SDK is shaped the way it is, see [../DESIGN.md](../DESIGN.md)
instead -- these guides assume that shape and just show you how to use
it.

- [Quickstart](./quickstart.md) -- stand up a minimal tool from scratch
- [Platforms and Keys](./platforms-and-keys.md) -- registering platforms,
  deployments, and managing signing keys
- [PostgreSQL Store](./postgres-store.md) -- using `pgstore` for real
  persistence, and how its schema maps to ltijs's
- [Handling a Launch](./handling-a-launch.md) -- reading claims, roles,
  and context out of a verified launch
- [Deep Linking](./deep-linking.md) -- returning content items to a
  platform's content picker
- [Grades and Roster (AGS/NRPS)](./grades-and-roster.md) -- posting
  scores and reading the class list
- [Dynamic Registration](./dynamic-registration.md) -- letting a platform
  admin self-register your tool
- [Testing Your Tool](./testing.md) -- `memstore`, `storetest`, and
  `ltitest` in your own test suite
- [Errors](./errors.md) -- the error model and how to branch on it
