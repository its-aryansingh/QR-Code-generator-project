import {
    Area,
    AreaChart,
    CartesianGrid,
    ResponsiveContainer,
    Tooltip,
    XAxis,
    YAxis,
} from "recharts";

interface AnalyticsChartProps {
    data: { date: string; count: number }[];
}

export function AnalyticsChart({ data }: AnalyticsChartProps) {
    if (!data || data.length === 0) {
        return (
            <div className="h-[300px] w-full flex items-center justify-center text-slate-500 bg-zinc-50/5 rounded-xl border border-zinc-50/10">
                No data available for this period
            </div>
        );
    }

    // Format dates for display
    const formattedData = data.map((item) => ({
        ...item,
        formattedDate: new Date(item.date).toLocaleDateString(undefined, {
            month: "short",
            day: "numeric",
        }),
    }));

    return (
        <div className="h-[300px] w-full bg-zinc-900 border border-zinc-50/10 rounded-xl p-4">
            <h3 className="text-lg font-semibold text-zinc-50 mb-4">Scan Activity</h3>
            <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={formattedData}>
                    <defs>
                        <linearGradient id="colorCount" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.3} />
                            <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0} />
                        </linearGradient>
                    </defs>
                    <CartesianGrid strokeDasharray="3 3" stroke="var(--color-zinc-800)" vertical={false} />
                    <XAxis
                        dataKey="formattedDate"
                        stroke="var(--color-zinc-500)"
                        fontSize={12}
                        tickLine={false}
                        axisLine={false}
                        tickMargin={10}
                    />
                    <YAxis
                        stroke="var(--color-zinc-500)"
                        fontSize={12}
                        tickLine={false}
                        axisLine={false}
                        tickFormatter={(value) => `${value}`}
                    />
                    <Tooltip
                        contentStyle={{ backgroundColor: "var(--color-zinc-900)", border: "1px solid var(--color-zinc-800)" }}
                        itemStyle={{ color: "var(--color-zinc-200)" }}
                        labelStyle={{ color: "var(--color-zinc-400)" }}
                        cursor={{ stroke: "#8b5cf6", strokeWidth: 1 }}
                    />
                    <Area
                        type="monotone"
                        dataKey="count"
                        stroke="#8b5cf6"
                        strokeWidth={3}
                        fillOpacity={1}
                        fill="url(#colorCount)"
                    />
                </AreaChart>
            </ResponsiveContainer>
        </div>
    );
}
