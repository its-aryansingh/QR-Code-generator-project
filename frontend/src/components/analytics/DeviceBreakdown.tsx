import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip, Legend } from "recharts";

interface DeviceBreakdownProps {
    deviceData?: { device_type: string; count: number }[];
    osData?: { os: string; count: number }[];
    browserData?: { browser: string; count: number }[];
}

const COLORS = ["#8b5cf6", "#ec4899", "#06b6d4", "#f59e0b", "#10b981", "#6366f1"];

export function DeviceBreakdown({ deviceData, osData, browserData }: DeviceBreakdownProps) {
    const renderChart = (title: string, data: any[], dataKey: string) => {
        if (!data || data.length === 0) return null;

        return (
            <div className="bg-[#12121a] border border-white/10 rounded-xl p-4 flex flex-col h-[300px]">
                <h3 className="text-md font-semibold text-white mb-2">{title}</h3>
                <div className="flex-1 w-full relative">
                    <ResponsiveContainer width="100%" height="100%">
                        <PieChart>
                            <Pie
                                data={data}
                                cx="50%"
                                cy="50%"
                                innerRadius={60}
                                outerRadius={80}
                                paddingAngle={5}
                                dataKey="count"
                                nameKey={dataKey}
                            >
                                {data.map((entry, index) => (
                                    <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} stroke="none" />
                                ))}
                            </Pie>
                            <Tooltip
                                contentStyle={{ backgroundColor: "#1e1e2e", border: "1px solid #334155", borderRadius: "8px" }}
                                itemStyle={{ color: "#e2e8f0" }}
                            />
                            <Legend
                                layout="horizontal"
                                verticalAlign="bottom"
                                align="center"
                                wrapperStyle={{ paddingTop: "20px", fontSize: "12px" }}
                            />
                        </PieChart>
                    </ResponsiveContainer>
                    {/* Total Count in Center */}
                    <div className="absolute top-1/2 left-1/2 transform -translate-x-1/2 -translate-y-[60%] text-center pointer-events-none">
                        <div className="text-2xl font-bold text-white">
                            {data.reduce((acc, item) => acc + item.count, 0)}
                        </div>
                        <div className="text-xs text-slate-400 uppercase tracking-wider">Total</div>
                    </div>
                </div>
            </div>
        );
    };

    return (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {renderChart("Device Type", deviceData || [], "device_type")}
            {renderChart("Operating System", osData || [], "os")}
            {renderChart("Browser", browserData || [], "browser")}
        </div>
    );
}
