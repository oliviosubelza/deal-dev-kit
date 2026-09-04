---
description: Generate a Zod schema for a feature in the web app
---

Ask for the target feature (under `src/features/<feature>/`) and the shape to validate, if not already given in the conversation.

Write the schema into that feature's `schemas/` folder:

```ts
import { z } from "zod";

export const orderSchema = z.object({
  // ...
});

export type Order = z.infer<typeof orderSchema>;
```

Follow the `web-architecture` skill for where a feature's slice lives and what belongs in `schemas/`.

## Out of scope

Cross-repo schema sharing with `crm-deal-mobile` is undefined and out of scope. Do not copy, sync, publish, or link this file to another repository — write only the local `features/<feature>/schemas/` file.
