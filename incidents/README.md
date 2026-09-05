# Incident evidence

Copy a file from `templates/` for each real incident or completed drill. Templates are examples, not proof that an exercise occurred. Store completed records in a deployment-owned evidence repository if incident details are sensitive; this directory defines the portable format.

Validate and summarize records with:

```bash
ecommerce-ops incident validate record.yaml
ecommerce-ops incident summary records/*.yaml
```

The summary reports overall and quarterly mean time to detect (MTTD), mean time to recover (MTTR), and open corrective actions. The validator rejects production drill records, invalid chronology, incomplete Sev1/Sev2 evidence, and corrective actions without owners and due dates.
