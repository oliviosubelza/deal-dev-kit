---
description: Generate a NestJS DTO from a Zod schema for the current module, using nestjs-zod's createZodDto
---

Ask for the target module (under `src/modules/<domain>/`) and the schema shape if not already given in the conversation.

Write the DTO into that module's `interface/dto/` folder, using `nestjs-zod`'s `createZodDto`:

```ts
import { createZodDto } from "nestjs-zod";
import { z } from "zod";

const OrderSchema = z.object({
  // ...
});

export class OrderDto extends createZodDto(OrderSchema) {}
```

Follow the `backend-architecture` skill for where the module and its `interface/` layer live.

## Out of scope

Cross-repo schema sharing between this service, `crm-deal-web`, and `crm-deal-mobile` is undefined and out of scope. Do not copy, sync, publish, or link this file to another repository — write only the local `interface/dto/` file.
