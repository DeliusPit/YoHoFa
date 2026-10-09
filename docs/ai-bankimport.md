# AI bij bankimport

De actieve v3-bankimport slaat een definitieve CSV-import eerst op als niet-geclassificeerde transacties. De afzonderlijke actie `POST /api/v1/transactions/classify-ai` gebruikt daarna de ingestelde `TRANSACTION_AI_URL`. Deze URL moet wijzen naar een OpenAI-compatible chat-completions endpoint, bijvoorbeeld `http://ai.example.local:8080/v1/chat/completions`. Na het invullen van deze URL kan de applicatie de beschikbare modellen ophalen via het bijbehorende `/v1/models` endpoint. Het gekozen model komt uit `TRANSACTION_AI_MODEL` of uit de applicatieconfiguratie. Preview blijft lokaal en deterministisch: de preview parseert alleen CSV via de gekozen bankconfiguratie.

Bij classificatie stuurt de API bestaande, niet-geclassificeerde transacties en financiële objecten in batches naar de transactie-AI. De AI mag alleen bestaande financiële objecten teruggeven; de app maakt geen categorieen/objecten aan op basis van AI-output.

De batches zijn klein gehouden zodat lokale modellen en `llama-swap` niet meteen grote prompts hoeven te verwerken. De chat-completions request gebruikt `stream:false` en `max_tokens:2048`.

## Request

De server doet een `POST` naar `TRANSACTION_AI_URL` met JSON zoals:

```json
{
  "model": "Qwen3.6-35B-A3B-Q4_K_M",
  "temperature": 0,
  "response_format": {"type": "json_object"},
  "messages": [
    {
      "role": "system",
      "content": "Classificeer banktransacties naar exact een van de aangeboden financiele objecten. Verzin geen nieuwe objecten. Geef uitsluitend JSON terug."
    },
    {
      "role": "user",
      "content": "Bankconfiguratie, financiele objecten en CSV-transacties..."
    }
  ]
}
```

De user-prompt bevat de bankconfiguratie, alle financiële objecten en alle te importeren CSV-regels. Financiële objecten zijn de categorieen uit de v3-kern; `objectType` is een van `asset`, `debt`, `expense`, `reserve` of `financial`.

## Response

De chat-completions response moet JSON in `choices[0].message.content` bevatten:

```json
{
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "{\"classifications\":[{\"row\":2,\"financialObjectId\":1,\"confidence\":0.95,\"reason\":\"Supermarktbetaling\"}]}"
      }
    }
  ]
}
```

Optioneel mag `amountCents` worden meegegeven voor een gedeeltelijke allocatie:

De inhoud van `message.content` is dus een JSON-string met `classifications`. Optioneel mag `amountCents` worden meegegeven voor een gedeeltelijke allocatie.

Als `amountCents` ontbreekt, gebruikt de import het volledige transactiebedrag als allocatiebedrag.

## Validatie

- Definitieve import vereist een geconfigureerde `TRANSACTION_AI_URL`.
- Het model is configureerbaar. De UI haalt beschikbare modellen op uit `/v1/models`; standaard is `Qwen3.6-35B-A3B-Q4_K_M`.
- `financialObjectId` moet bestaan in de meegegeven categorieen.
- Een rij zonder classificatie wordt wel geimporteerd, maar blijft ongealloceerd.
- Ongeldige AI-output of een niet-bereikbare AI blokkeert de definitieve import.
- Bij een foutstatus van de AI-server toont de import een beperkte fouttekst uit de AI-response, zodat model-, context- of requestfouten zichtbaar zijn.
- De ruwe bankregel blijft opgeslagen in `bank_import_rows`.
