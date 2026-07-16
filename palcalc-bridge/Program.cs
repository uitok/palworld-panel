using System.Collections.Concurrent;
using System.Text.Json.Serialization;
using PalCalc.Model;
using PalCalc.Solver;
using PalCalc.Solver.PalReference;
using PalCalc.Solver.ResultPruning;

Logging.InitCommonFull();
var palDb = PalDB.LoadEmbedded();
_ = PalBreedingDB.LoadEmbedded(palDb);

var builder = WebApplication.CreateBuilder(args);
builder.Services.ConfigureHttpJsonOptions(options =>
{
    options.SerializerOptions.PropertyNamingPolicy = System.Text.Json.JsonNamingPolicy.SnakeCaseLower;
    options.SerializerOptions.DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull;
    options.SerializerOptions.Converters.Add(new JsonStringEnumConverter());
});
builder.WebHost.UseUrls(Environment.GetEnvironmentVariable("PALCALC_BRIDGE_URLS") ?? "http://127.0.0.1:8091");

var app = builder.Build();
var jobs = new ConcurrentDictionary<string, SolverJob>();
var gate = new SemaphoreSlim(Math.Max(1, IntEnv("PALCALC_BRIDGE_CONCURRENCY", 1)));

app.MapGet("/health", () => Results.Ok(new
{
    status = "ok",
    upstream = "tylercamp/palcalc",
    upstream_version = "v1.17.6",
    upstream_commit = "8b7e2f779e47fddae16ddcb973e828ba20c02b80",
    database_version = palDb.Version,
}));

app.MapGet("/v1/catalog", () => Results.Ok(new CatalogResponse(
    palDb.Version,
    palDb.Pals.OrderBy(p => p.Name).Select(p => new PalCatalogItem(p.InternalName, p.Name)).ToList(),
    palDb.StandardPassiveSkills.OrderBy(p => p.Name).Select(p => new PassiveCatalogItem(p.InternalName, p.Name, p.SupportsSurgery, p.SurgeryCost)).ToList(),
    palDb.ActiveSkills.OrderBy(p => p.Name).Select(p => new SkillCatalogItem(p.InternalName, p.Name)).ToList()
)));

app.MapPost("/v1/jobs", (SolveRequest request) =>
{
    try
    {
        ValidateRequest(request, palDb);
    }
    catch (Exception ex)
    {
        return Results.BadRequest(new { code = "invalid_request", message = ex.Message });
    }

    var id = string.IsNullOrWhiteSpace(request.RequestId) ? Guid.NewGuid().ToString("N") : request.RequestId.Trim();
    if (jobs.TryGetValue(id, out var existing)) return Results.Ok(existing.Snapshot());

    var job = new SolverJob(id, request);
    if (!jobs.TryAdd(id, job)) return Results.Conflict(new { code = "job_exists", message = "job id already exists" });
    _ = RunJob(job, palDb, gate);
    return Results.Accepted($"/v1/jobs/{id}", job.Snapshot());
});

app.MapGet("/v1/jobs/{id}", (string id) =>
    jobs.TryGetValue(id, out var job) ? Results.Ok(job.Snapshot()) : Results.NotFound(new { code = "job_not_found" }));

app.MapGet("/v1/jobs/{id}/result", (string id) =>
{
    if (!jobs.TryGetValue(id, out var job)) return Results.NotFound(new { code = "job_not_found" });
    if (job.Status != "completed") return Results.Conflict(new { code = "job_not_completed", status = job.Status });
    return Results.Ok(job.Result);
});

app.MapPost("/v1/jobs/{id}/pause", (string id) =>
{
    if (!jobs.TryGetValue(id, out var job)) return Results.NotFound(new { code = "job_not_found" });
    job.Controller.Pause();
    job.Status = "paused";
    job.UpdatedAt = DateTimeOffset.UtcNow;
    return Results.Ok(job.Snapshot());
});

app.MapPost("/v1/jobs/{id}/resume", (string id) =>
{
    if (!jobs.TryGetValue(id, out var job)) return Results.NotFound(new { code = "job_not_found" });
    job.Controller.Resume();
    if (job.Status == "paused") job.Status = "running";
    job.UpdatedAt = DateTimeOffset.UtcNow;
    return Results.Ok(job.Snapshot());
});

app.MapDelete("/v1/jobs/{id}", (string id) =>
{
    if (!jobs.TryGetValue(id, out var job)) return Results.NotFound(new { code = "job_not_found" });
    job.Cancellation.Cancel();
    job.Controller.Resume();
    job.Status = "canceling";
    job.UpdatedAt = DateTimeOffset.UtcNow;
    return Results.Accepted($"/v1/jobs/{id}", job.Snapshot());
});

app.Run();

static async Task RunJob(SolverJob job, PalDB db, SemaphoreSlim gate)
{
    try
    {
        await gate.WaitAsync(job.Cancellation.Token);
        job.Status = "running";
        job.StartedAt = job.UpdatedAt = DateTimeOffset.UtcNow;

        var request = job.Request;
        var ownedPals = request.OwnedPals.Select(input => ToPalInstance(input, db)).ToList();
        var settings = request.Settings ?? new SolverSettingsInput();
        var game = request.GameSettings ?? new GameSettingsInput();
        var solver = new BreedingSolver(new BreedingSolverSettings(
            db: db,
            gameSettings: new GameSettings
            {
                BreedingTime = TimeSpan.FromSeconds(Math.Max(0, game.BreedingTimeSeconds)),
                MassiveEggIncubationTime = TimeSpan.FromMinutes(Math.Max(0, game.MassiveEggIncubationMinutes)),
                MultipleBreedingFarms = game.MultipleBreedingFarms,
                MultipleIncubators = game.MultipleIncubators,
            },
            ownedPals: ownedPals,
            pruningBuilder: PruningRulesBuilder.Default,
            maxBreedingSteps: Math.Clamp(settings.MaxBreedingSteps, 1, 99),
            maxSolverIterations: Math.Clamp(settings.MaxSolverIterations, 1, 99),
            maxWildPals: Math.Max(0, settings.MaxWildPals),
            allowedWildPals: SelectPals(db, settings.AllowedWildPals, settings.BannedWildPals),
            bannedBredPals: LookupPals(db, settings.BannedBredPals),
            maxInputIrrelevantPassives: Math.Clamp(settings.MaxInputIrrelevantPassives, 0, 4),
            maxBredIrrelevantPassives: Math.Clamp(settings.MaxBredIrrelevantPassives, 0, 4),
            maxEffort: TimeSpan.FromDays(3650),
            maxThreads: settings.MaxThreads,
            maxSurgeryCost: Math.Max(0, settings.MaxGoldCost),
            allowedSurgeryPassives: SelectPassives(db, settings.AllowedSurgeryPassives, settings.BannedSurgeryPassives),
            useGenderReversers: settings.UseGenderReversers
        ));
        solver.SolverStateUpdateInterval = TimeSpan.FromMilliseconds(250);
        solver.SolverStateUpdated += state =>
        {
            job.Phase = state.CurrentPhase.ToString().ToLowerInvariant();
            job.Step = state.CurrentStepIndex;
            job.TargetSteps = state.TargetSteps;
            job.WorkProcessed = state.WorkProcessedCount;
            job.WorkTotal = state.CurrentWorkSize;
            job.UpdatedAt = DateTimeOffset.UtcNow;
        };

        job.Controller.CancellationToken = job.Cancellation.Token;
        var target = request.Target;
        var specifier = new PalSpecifier
        {
            Pal = FindPal(db, target.PalId),
            RequiredGender = ParseGender(target.Gender),
            RequiredPassives = LookupPassives(db, target.RequiredPassives),
            OptionalPassives = LookupPassives(db, target.OptionalPassives),
            IV_HP = Math.Clamp(target.IvHp, 0, 100),
            IV_Attack = Math.Clamp(target.IvAttack, 0, 100),
            IV_Defense = Math.Clamp(target.IvDefense, 0, 100),
        };

        // PalCalc performs a CPU-heavy synchronous search. Run it on a dedicated
        // thread so Kestrel's worker pool remains responsive to progress and
        // cancellation requests while a solve is active.
        var matches = await Task.Factory.StartNew(
            () => solver.SolveFor(specifier, job.Controller),
            job.Cancellation.Token,
            TaskCreationOptions.LongRunning,
            TaskScheduler.Default
        );
        var limit = Math.Clamp(request.ResultLimit, 1, 100);
        job.Result = new SolveResponse(
            request.SaveFingerprint,
            matches.OrderBy(m => m.BreedingEffort).Take(limit).Select(ToResult).ToList()
        );
        job.Status = "completed";
        job.Phase = "finished";
    }
    catch (OperationCanceledException)
    {
        job.Status = "canceled";
        job.ErrorCode = "canceled";
        job.Error = "solver job was canceled";
    }
    catch (Exception ex)
    {
        job.Status = "failed";
        job.ErrorCode = "solver_failed";
        job.Error = ex.Message;
    }
    finally
    {
        job.FinishedAt = job.UpdatedAt = DateTimeOffset.UtcNow;
        if (job.StartedAt != null) gate.Release();
    }
}

static SolveResult ToResult(IPalReference result) => new(
    result.Pal.InternalName,
    result.Pal.Name,
    result.Gender.ToString().ToLowerInvariant(),
    result.EffectivePassives.Select(p => p.InternalName).ToList(),
    new IvRangeDto(ToIv(result.IVs.HP), ToIv(result.IVs.Attack), ToIv(result.IVs.Defense)),
    result.BreedingEffort.TotalSeconds,
    result.NumTotalBreedingSteps,
    result.NumTotalEggs,
    result.NumTotalWildPals,
    result.TotalCost,
    ToTree(result)
);

static TreeNodeDto ToTree(IPalReference value)
{
    var common = new TreeNodeDto
    {
        Type = value switch
        {
            OwnedPalReference => "owned",
            CompositeOwnedPalReference => "composite_owned",
            WildPalReference => "wild",
            BredPalReference => "bred",
            SurgeryTablePalReference => "surgery",
            _ => "unknown",
        },
        PalId = value.Pal.InternalName,
        PalName = value.Pal.Name,
        Gender = value.Gender.ToString().ToLowerInvariant(),
        Passives = value.EffectivePassives.Select(p => p.InternalName).ToList(),
        Ivs = new IvRangeDto(ToIv(value.IVs.HP), ToIv(value.IVs.Attack), ToIv(value.IVs.Defense)),
        EffortSeconds = value.BreedingEffort.TotalSeconds,
        SelfEffortSeconds = value.SelfBreedingEffort.TotalSeconds,
        Cost = value.TotalCost,
    };

    switch (value)
    {
        case OwnedPalReference owned:
            common.InstanceId = owned.UnderlyingInstance.InstanceId;
            common.OwnerPlayerId = owned.UnderlyingInstance.OwnerPlayerId;
            common.ContainerId = owned.UnderlyingInstance.Location?.ContainerId;
            common.SlotIndex = owned.UnderlyingInstance.Location?.Index;
            common.LocationType = owned.UnderlyingInstance.Location?.Type.ToString().ToLowerInvariant();
            break;
        case CompositeOwnedPalReference composite:
            common.Children = [ToTree(composite.Male), ToTree(composite.Female)];
            break;
        case BredPalReference bred:
            common.Probability = bred.PassivesProbability * bred.IVsProbability;
            common.Eggs = bred.AvgRequiredBreedings;
            common.Children = [ToTree(bred.Parent1), ToTree(bred.Parent2)];
            break;
        case SurgeryTablePalReference surgery:
            common.Operations = surgery.Operations.Select(operation => operation.ToString() ?? "surgery").ToList();
            common.Children = [ToTree(surgery.Input)];
            break;
    }
    return common;
}

static IvValueDto ToIv(IV_Value value) => new(value.IsRelevant, value.Min, value.Max);

static PalInstance ToPalInstance(OwnedPalInput input, PalDB db) => new()
{
    InstanceId = input.InstanceId,
    NickName = input.Nickname,
    Level = Math.Max(1, input.Level),
    OwnerPlayerId = input.OwnerPlayerId,
    Pal = FindPal(db, input.PalId),
    Gender = ParseGender(input.Gender),
    PassiveSkills = LookupPassives(db, input.Passives),
    ActiveSkills = [],
    EquippedActiveSkills = [],
    Rank = Math.Clamp(input.Rank, 1, 5),
    IV_HP = Math.Clamp(input.IvHp, 0, 100),
    IV_Shot = Math.Clamp(input.IvAttack, 0, 100),
    IV_Defense = Math.Clamp(input.IvDefense, 0, 100),
    Location = new PalLocation
    {
        ContainerId = input.ContainerId,
        Index = Math.Max(0, input.SlotIndex),
        Type = ParseLocation(input.LocationType),
    },
};

static Pal FindPal(PalDB db, string id) => db.Pals.FirstOrDefault(p =>
    p.InternalName.Equals(id, StringComparison.OrdinalIgnoreCase) || p.Name.Equals(id, StringComparison.OrdinalIgnoreCase))
    ?? throw new ArgumentException($"unknown pal: {id}");

static List<Pal> LookupPals(PalDB db, IEnumerable<string>? values) => (values ?? []).Select(value => FindPal(db, value)).Distinct().ToList();
static List<PassiveSkill> LookupPassives(PalDB db, IEnumerable<string>? values) => (values ?? []).Select(value =>
    db.StandardPassiveSkills.FirstOrDefault(p => p.InternalName.Equals(value, StringComparison.OrdinalIgnoreCase) || p.Name.Equals(value, StringComparison.OrdinalIgnoreCase))
    ?? throw new ArgumentException($"unknown passive: {value}")).Distinct().ToList();

static List<Pal> SelectPals(PalDB db, List<string>? allowed, List<string>? banned)
{
    var source = allowed is { Count: > 0 } ? LookupPals(db, allowed) : db.Pals.ToList();
    return source.Except(LookupPals(db, banned)).ToList();
}

static List<PassiveSkill> SelectPassives(PalDB db, List<string>? allowed, List<string>? banned)
{
    var source = allowed is { Count: > 0 } ? LookupPassives(db, allowed) : db.SurgeryPassiveSkills.ToList();
    return source.Except(LookupPassives(db, banned)).ToList();
}

static PalGender ParseGender(string? value) => value?.Trim().ToLowerInvariant() switch
{
    "male" or "m" => PalGender.MALE,
    "female" or "f" => PalGender.FEMALE,
    "opposite_wildcard" => PalGender.OPPOSITE_WILDCARD,
    "none" => PalGender.NONE,
    _ => PalGender.WILDCARD,
};

static LocationType ParseLocation(string? value) => value?.Trim().ToLowerInvariant() switch
{
    "player_party" or "party" => LocationType.PlayerParty,
    "base" => LocationType.Base,
    "viewing_cage" => LocationType.ViewingCage,
    "custom" => LocationType.Custom,
    "dimensional_pal_storage" or "dimensional" => LocationType.DimensionalPalStorage,
    "global_pal_storage" or "global" => LocationType.GlobalPalStorage,
    _ => LocationType.Palbox,
};

static void ValidateRequest(SolveRequest request, PalDB db)
{
    if (request.Target == null) throw new ArgumentException("target is required");
    _ = FindPal(db, request.Target.PalId);
    _ = LookupPassives(db, request.Target.RequiredPassives);
    _ = LookupPassives(db, request.Target.OptionalPassives);
    if (request.OwnedPals == null) throw new ArgumentException("owned_pals is required");
    foreach (var pal in request.OwnedPals) _ = FindPal(db, pal.PalId);
}

static int IntEnv(string name, int fallback) => int.TryParse(Environment.GetEnvironmentVariable(name), out var value) ? value : fallback;

sealed class SolverJob(string id, SolveRequest request)
{
    public string Id { get; } = id;
    public SolveRequest Request { get; } = request;
    public string Status { get; set; } = "queued";
    public string Phase { get; set; } = "queued";
    public int Step { get; set; }
    public int TargetSteps { get; set; }
    public long WorkProcessed { get; set; }
    public long WorkTotal { get; set; }
    public string? ErrorCode { get; set; }
    public string? Error { get; set; }
    public DateTimeOffset CreatedAt { get; } = DateTimeOffset.UtcNow;
    public DateTimeOffset UpdatedAt { get; set; } = DateTimeOffset.UtcNow;
    public DateTimeOffset? StartedAt { get; set; }
    public DateTimeOffset? FinishedAt { get; set; }
    public SolveResponse? Result { get; set; }
    public CancellationTokenSource Cancellation { get; } = new(TimeSpan.FromMinutes(Math.Max(1, ReadTimeoutMinutes())));
    public SolverStateController Controller { get; } = new();

    public object Snapshot() => new
    {
        id = Id, status = Status, phase = Phase, step = Step, target_steps = TargetSteps,
        work_processed = WorkProcessed, work_total = WorkTotal, error_code = ErrorCode, error = Error,
        created_at = CreatedAt, updated_at = UpdatedAt, started_at = StartedAt, finished_at = FinishedAt,
        result_count = Result?.Results.Count ?? 0,
    };

    private static int ReadTimeoutMinutes() =>
        int.TryParse(Environment.GetEnvironmentVariable("PALCALC_BRIDGE_TIMEOUT_MINUTES"), out var value) ? value : 5;
}

record CatalogResponse(string Version, List<PalCatalogItem> Pals, List<PassiveCatalogItem> Passives, List<SkillCatalogItem> ActiveSkills);
record PalCatalogItem(string Id, string Name);
record PassiveCatalogItem(string Id, string Name, bool SupportsSurgery, int SurgeryCost);
record SkillCatalogItem(string Id, string Name);
record SolveRequest(string? RequestId, string SaveFingerprint, List<OwnedPalInput> OwnedPals, TargetInput Target, SolverSettingsInput? Settings, GameSettingsInput? GameSettings, int ResultLimit = 20);
record OwnedPalInput(string InstanceId, string PalId, string? Nickname, int Level, string OwnerPlayerId, string Gender, List<string> Passives, int Rank, int IvHp, int IvAttack, int IvDefense, string ContainerId, int SlotIndex, string LocationType);
record TargetInput(string PalId, string? Gender, List<string>? RequiredPassives, List<string>? OptionalPassives, int IvHp, int IvAttack, int IvDefense);
record SolverSettingsInput(
    int MaxBreedingSteps = 6, int MaxSolverIterations = 20, int MaxWildPals = 1,
    int MaxInputIrrelevantPassives = 2, int MaxBredIrrelevantPassives = 1, int MaxThreads = 0,
    int MaxGoldCost = 0, bool UseGenderReversers = false, List<string>? AllowedWildPals = null,
    List<string>? BannedWildPals = null, List<string>? BannedBredPals = null,
    List<string>? AllowedSurgeryPassives = null, List<string>? BannedSurgeryPassives = null);
record GameSettingsInput(int BreedingTimeSeconds = 300, int MassiveEggIncubationMinutes = 120, bool MultipleBreedingFarms = true, bool MultipleIncubators = true);
record SolveResponse(string SaveFingerprint, List<SolveResult> Results);
record SolveResult(string PalId, string PalName, string Gender, List<string> Passives, IvRangeDto Ivs, double EffortSeconds, int BreedingSteps, int Eggs, int WildPals, int GoldCost, TreeNodeDto Tree);
record IvValueDto(bool Relevant, int Min, int Max);
record IvRangeDto(IvValueDto Hp, IvValueDto Attack, IvValueDto Defense);

sealed class TreeNodeDto
{
    public string Type { get; set; } = "unknown";
    public string PalId { get; set; } = "";
    public string PalName { get; set; } = "";
    public string Gender { get; set; } = "wildcard";
    public List<string> Passives { get; set; } = [];
    public IvRangeDto? Ivs { get; set; }
    public double EffortSeconds { get; set; }
    public double SelfEffortSeconds { get; set; }
    public int Cost { get; set; }
    public float? Probability { get; set; }
    public int? Eggs { get; set; }
    public string? InstanceId { get; set; }
    public string? OwnerPlayerId { get; set; }
    public string? ContainerId { get; set; }
    public int? SlotIndex { get; set; }
    public string? LocationType { get; set; }
    public List<string>? Operations { get; set; }
    public List<TreeNodeDto>? Children { get; set; }
}
